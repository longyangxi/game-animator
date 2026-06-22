package sprite

import (
	"image"
	"image/draw"
	"math"
)

// Adaptive matting parameters based on the chroma (CbCr) plane.
// Luma (Y) is ignored and the key is separated by saturation/hue alone, so it is robust to JPEG 4:2:0 compression.
const (
	chromaIn     = 24.0  // CbCr distance at or below this → fully transparent (key color)
	chromaOut    = 72.0  // CbCr distance at or above this → fully opaque (subject)
	despillBand  = 100.0 // pixels within this distance are candidates for key-tint spill (despill) correction
	despillScale = 0.92  // despill strength (saturation suppression rate toward the key)
	floodTol     = 88.0  // CbCr distance below which the border-seed flood fill treats a pixel as background (lenient)
)

// ycc holds BT.601 YCbCr coordinates (8-bit, centered at 128).
type ycc struct{ y, cb, cr float64 }

func toYCC(r, g, b uint8) ycc {
	fr, fg, fb := float64(r), float64(g), float64(b)
	y := 0.299*fr + 0.587*fg + 0.114*fb
	return ycc{
		y:  y,
		cb: (fb-y)*0.564 + 128,
		cr: (fr-y)*0.713 + 128,
	}
}

func fromYCC(c ycc) (uint8, uint8, uint8) {
	r := c.y + 1.402*(c.cr-128)
	g := c.y - 0.344136*(c.cb-128) - 0.714136*(c.cr-128)
	b := c.y + 1.772*(c.cb-128)
	return u8(r), u8(g), u8(b)
}

func u8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// smoothstep produces a smooth 0→1 transition via Hermite interpolation (edge feathering).
func smoothstep(edge0, edge1, x float64) float64 {
	if edge1 <= edge0 {
		return 0
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// ToNRGBA converts an arbitrary image to a top-left-origin NRGBA.
func ToNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	dst := image.NewNRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

// DetectBackground estimates the background key color from the most frequent chroma (CbCr)
// cluster of the border pixels. Rather than a plain RGB average, it finds the mode of a
// quantized CbCr-plane histogram, so it reliably captures the dominant color even on
// backgrounds with gradients or compression noise.
func DetectBackground(img *image.NRGBA) [3]uint8 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return [3]uint8{255, 0, 255}
	}
	type acc struct {
		n          int
		sr, sg, sb int
	}
	bins := map[int]*acc{}
	total := 0
	var magN, magR, magG, magB int // accumulate magenta-family pixels (strong R·B, weak G)
	visit := func(x, y int) {
		i := img.PixOffset(x, y)
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		total++
		if r > 150 && b > 150 && g < 120 { // magenta family
			magN++
			magR += int(r)
			magG += int(g)
			magB += int(b)
		}
		c := toYCC(r, g, b)
		key := int(c.cb)>>3<<6 | int(c.cr)>>3 // CbCr quantized in steps of 8
		a := bins[key]
		if a == nil {
			a = &acc{}
			bins[key] = a
		}
		a.n++
		a.sr += int(r)
		a.sg += int(g)
		a.sb += int(b)
	}
	// Wide poses (walking, etc.) touch the entire border, so the character color contaminates
	// the key estimate. The corner square patches are almost always background, so sample
	// primarily around the corners.
	cw, ch := w/5, h/5
	if cw < 2 {
		cw = w
	}
	if ch < 2 {
		ch = h
	}
	corner := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				visit(x, y)
			}
		}
	}
	corner(0, 0, cw, ch)
	corner(w-cw, 0, w, ch)
	corner(0, h-ch, cw, h)
	corner(w-cw, h-ch, w, h)
	// Also use the thin border as a fallback (for the rare case where corners are covered by the character)
	for x := 0; x < w; x++ {
		visit(x, 0)
		visit(x, h-1)
	}
	for y := 0; y < h; y++ {
		visit(0, y)
		visit(w-1, y)
	}
	// Magenta bias: this pipeline always intends a magenta key, so if the magenta family is
	// sufficiently present (12%+ of samples) on the border/corners — even when a wide or
	// reclining pose fills the corners and makes the overall mode the character color — fix
	// the magenta cluster as the key.
	if total > 0 && magN >= total*12/100 {
		return [3]uint8{uint8(magR / magN), uint8(magG / magN), uint8(magB / magN)}
	}
	var best *acc
	for _, a := range bins {
		if best == nil || a.n > best.n {
			best = a
		}
	}
	if best == nil || best.n == 0 {
		return [3]uint8{255, 0, 255}
	}
	return [3]uint8{uint8(best.sr / best.n), uint8(best.sg / best.n), uint8(best.sb / best.n)}
}

// RemoveBackground auto-detects the background key and makes it transparent via chroma-plane matting.
// (1) a soft alpha ramp based on CbCr distance → edge feathering, (2) chroma-space despill to
// remove key-tint spill, (3) morphological cleanup of isolated dots/pinholes.
func RemoveBackground(src image.Image) *image.NRGBA {
	img := ToNRGBA(src)
	key := DetectBackground(img)
	out, frac := matteWith(img, key)

	// Safeguard: if a wide pose fills the border and the key is mistaken for the character color,
	// matting either erases the character instead of the background (opaque ratio spikes) or
	// removes only part of the magenta background (magenta residue spikes). This pipeline always
	// intends a magenta key, so if either symptom appears, re-matte as a fallback with pure
	// magenta (#FF00FF) and take whichever is better.
	if frac > 0.60 || magentaResidueFrac(out) > 0.025 {
		out2, frac2 := matteWith(img, [3]uint8{255, 0, 255})
		betterFrac := frac2 < frac-0.03 && frac2 > 0.02
		lessResidue := magentaResidueFrac(out2) < magentaResidueFrac(out)
		if (betterFrac || lessResidue) && frac2 > 0.02 {
			out = out2
		}
	}
	// Search/dark-background fallback: if a dark/achromatic key is detected, also try pure
	// magenta matting and take whichever is better (less magenta residue).
	if !isMagentaKey(key) {
		out2, frac2 := matteWith(img, [3]uint8{255, 0, 255})
		if frac2 > 0.02 && magentaResidueFrac(out2) < magentaResidueFrac(out) {
			out = out2
		}
	}
	cleanupAlpha(out)
	return out
}

// magentaResidueFrac is the fraction of opaque pixels close to pure magenta (CbCr<55) out of the total.
// It is a symptom indicator for whether matting fully removed the magenta background.
func magentaResidueFrac(img *image.NRGBA) float64 {
	mk := toYCC(255, 0, 255)
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i+3] <= alphaThreshold {
			continue
		}
		c := toYCC(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
		if math.Hypot(c.cb-mk.cb, c.cr-mk.cr) < 55 {
			n++
		}
	}
	total := len(img.Pix) / 4
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

// isMagentaKey reports whether the key color is in the magenta family (strong R·B, weak G).
func isMagentaKey(k [3]uint8) bool {
	return k[0] > 150 && k[2] > 150 && k[1] < 120
}

// matteWith performs chroma-plane matting + despill + flood fill with the given key,
// and returns the result image and the ratio of opaque (alpha>threshold) pixels.
func matteWith(img *image.NRGBA, key [3]uint8) (*image.NRGBA, float64) {
	kc := toYCC(key[0], key[1], key[2])
	kvb, kvr := kc.cb-128, kc.cr-128
	klen := math.Hypot(kvb, kvr)
	out := image.NewNRGBA(img.Rect)

	for i := 0; i+3 < len(img.Pix); i += 4 {
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if a == 0 {
			continue
		}
		c := toYCC(r, g, b)
		dist := math.Hypot(c.cb-kc.cb, c.cr-kc.cr)
		alpha := smoothstep(chromaIn, chromaOut, dist)
		if alpha <= 0 {
			continue
		}
		if klen > 1 && dist < despillBand {
			pcb, pcr := c.cb-128, c.cr-128
			proj := (pcb*kvb + pcr*kvr) / klen
			if proj > 0 {
				wgt := smoothstep(0, 1, (despillBand-dist)/despillBand) * despillScale
				ub, ur := kvb/klen, kvr/klen
				c.cb = 128 + (pcb - ub*proj*wgt)
				c.cr = 128 + (pcr - ur*proj*wgt)
				r, g, b = fromYCC(c)
			}
		}
		out.Pix[i] = r
		out.Pix[i+1] = g
		out.Pix[i+2] = b
		out.Pix[i+3] = uint8(float64(a) * alpha)
	}

	floodClearBackground(out, img, kc)

	opaque := 0
	for i := 3; i < len(out.Pix); i += 4 {
		if out.Pix[i] > alphaThreshold {
			opaque++
		}
	}
	frac := float64(opaque) / float64(len(out.Pix)/4)
	return out, frac
}

// floodClearBackground starts from the border and 4-way flood fills along pixels close to the
// key color (lenient tolerance), setting alpha to 0. It reliably removes, by connectivity,
// gradient/noise backgrounds that soft matting alone cannot erase (such as when a wide pose
// like walking destabilizes the key), while preserving interior character pixels that are
// disconnected from the border (even if they are the key color).
func floodClearBackground(out *image.NRGBA, orig *image.NRGBA, kc ycc) {
	w, h := orig.Rect.Dx(), orig.Rect.Dy()
	if w < 3 || h < 3 {
		return
	}
	isKey := func(x, y int) bool {
		i := orig.PixOffset(x, y)
		c := toYCC(orig.Pix[i], orig.Pix[i+1], orig.Pix[i+2])
		return math.Hypot(c.cb-kc.cb, c.cr-kc.cr) <= floodTol
	}
	visited := make([]bool, w*h)
	stack := make([]int, 0, 4096)
	push := func(x, y int) {
		p := y*w + x
		if !visited[p] && isKey(x, y) {
			visited[p] = true
			stack = append(stack, p)
		}
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := p%w, p/w
		out.Pix[p*4+3] = 0 // background → transparent
		if x > 0 {
			push(x-1, y)
		}
		if x < w-1 {
			push(x+1, y)
		}
		if y > 0 {
			push(x, y-1)
		}
		if y < h-1 {
			push(x, y+1)
		}
	}
}

// cleanupAlpha removes isolated opaque dots (JPEG block speckle) and fills 1px pinholes.
// To preserve soft edges, it only touches clearly isolated/surrounded pixels.
func cleanupAlpha(img *image.NRGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w < 3 || h < 3 {
		return
	}
	orig := make([]uint8, w*h)
	for p := 0; p < w*h; p++ {
		orig[p] = img.Pix[p*4+3]
	}
	opaque := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		if orig[y*w+x] > alphaThreshold {
			return 1
		}
		return 0
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			nb := opaque(x-1, y) + opaque(x+1, y) + opaque(x, y-1) + opaque(x, y+1) +
				opaque(x-1, y-1) + opaque(x+1, y-1) + opaque(x-1, y+1) + opaque(x+1, y+1)
			if orig[y*w+x] > alphaThreshold {
				if nb == 0 { // fully isolated dot → remove
					img.Pix[i+3] = 0
				}
			} else if nb >= 7 { // nearly surrounded pinhole → fill
				img.Pix[i+3] = 255
			}
		}
	}
}

// colorDist is the RGB Euclidean distance (for inspect's residual-chroma decision).
func colorDist(r, g, b uint8, bg [3]uint8) float64 {
	dr := float64(r) - float64(bg[0])
	dg := float64(g) - float64(bg[1])
	db := float64(b) - float64(bg[2])
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
