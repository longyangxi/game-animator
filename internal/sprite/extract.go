package sprite

import (
	"fmt"
	"image"

	xdraw "golang.org/x/image/draw"
)

const alphaThreshold = 10 // alpha at or below this value is treated as an empty (transparent) pixel

// frameContent is the content of a single pose extracted in strip coordinates.
type frameContent struct {
	img    *image.NRGBA // content cropped to its bbox
	minX   int
	cx     float64 // alpha-weighted center of mass X (strip coordinates)
	bottom int     // baseline (lowest content row, strip coordinates)
}

// extractContent crops the opaque pixels within the column span to a bbox.
// It gathers all content in the span without ownership tracking (connected components), so even
// if limbs are detached they are safely merged into one pose.
func extractContent(strip *image.NRGBA, span colSpan, h int) frameContent {
	minX, minY, maxX, maxY := span.end, h, span.start-1, -1
	var sumWX, sumW float64
	for x := span.start; x < span.end; x++ {
		for y := 0; y < h; y++ {
			a := strip.Pix[strip.PixOffset(x, y)+3]
			if a <= alphaThreshold {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
			sumWX += float64(x) * float64(a)
			sumW += float64(a)
		}
	}
	if maxX < minX || maxY < minY {
		return frameContent{}
	}
	gw, gh := maxX-minX+1, maxY-minY+1
	dst := image.NewNRGBA(image.Rect(0, 0, gw, gh))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			si := strip.PixOffset(x, y)
			if strip.Pix[si+3] <= alphaThreshold {
				continue
			}
			di := dst.PixOffset(x-minX, y-minY)
			copy(dst.Pix[di:di+4], strip.Pix[si:si+4])
		}
	}
	cx := float64(minX+maxX+1) / 2
	if sumW > 0 {
		cx = sumWX / sumW
	}
	return frameContent{img: dst, minX: minX, cx: cx, bottom: maxY}
}

// ExtractFrames detects poses in a transparent-background strip via projection segmentation and
// produces cell-sized frames. It applies a common scale to all frames, aligns them horizontally
// by center of mass, and preserves vertical offsets (jump arcs, etc.) against a common baseline.
func ExtractFrames(strip *image.NRGBA, expected, cellW, cellH, margin int) ExtractResult {
	res := ExtractResult{Expected: expected}
	segs, natural := segmentStrip(strip, expected)
	if len(segs) == 0 {
		res.Warnings = append(res.Warnings, "No character was found in the image. Please regenerate.")
		return res
	}
	h := strip.Rect.Dy()

	var fcs []frameContent
	for _, s := range segs {
		fc := extractContent(strip, s, h)
		if fc.img != nil {
			fcs = append(fcs, fc)
		}
	}
	if len(fcs) == 0 {
		res.Warnings = append(res.Warnings, "No valid pose was found. Please regenerate.")
		return res
	}

	// common baseline + shared scale
	baseline := 0
	for _, g := range fcs {
		if g.bottom > baseline {
			baseline = g.bottom
		}
	}
	availW := cellW - margin*2
	availH := cellH - margin*2
	if availW < 8 || availH < 8 {
		availW, availH = cellW, cellH
	}
	maxBodyW, maxBodyH := 1, 1
	for _, g := range fcs {
		bw, bh := bodyExtent(g.img)
		if bw > maxBodyW {
			maxBodyW = bw
		}
		if bh > maxBodyH {
			maxBodyH = bh
		}
	}
	scale := minf(float64(availW)/float64(maxBodyW), float64(availH)/float64(maxBodyH))
	if scale > 1 {
		scale = 1
	}

	for _, g := range fcs {
		// scale was computed against the body extent, so adjust the remaining empty space in the
		// bounding box to fit the available space.
		boxScale := minf(scale, minf(float64(availW)/float64(g.img.Rect.Dx()), float64(availH)/float64(g.img.Rect.Dy())))
		if boxScale > 1 {
			boxScale = 1
		}
		// for vertical alignment, use the content's bottom rather than the actual feet/lower-body (bottom)
		sw := int(float64(g.img.Rect.Dx())*boxScale + 0.5)
		sh := int(float64(g.img.Rect.Dy())*boxScale + 0.5)
		if sw < 1 {
			sw = 1
		}
		if sh < 1 {
			sh = 1
		}
		scaled := g.img
		if sw != g.img.Rect.Dx() || sh != g.img.Rect.Dy() {
			scaled = image.NewNRGBA(image.Rect(0, 0, sw, sh))
			xdraw.CatmullRom.Scale(scaled, scaled.Rect, g.img, g.img.Rect, xdraw.Over, nil)
		}
		// scale the offset relative to the content's bottom in the strip, for common-baseline correction
		contentBaseline := int(float64(baseline-g.bottom)*boxScale + 0.5)

		cell := image.NewNRGBA(image.Rect(0, 0, cellW, cellH))
		// place horizontally so the center of mass lands at the cell center (even if limbs extend to
		// one side, the larger-area torso dominates, so there is little wobble between frames).
		left := int(float64(cellW)/2 - (g.cx-float64(g.minX))*boxScale + 0.5)
		if left < 0 {
			left = 0
		}
		if left+sw > cellW {
			left = cellW - sw
		}
		top := cellH - margin - contentBaseline - sh
		if top < 0 {
			top = 0
		}
		xdraw.Copy(cell, image.Point{X: left, Y: top}, scaled, scaled.Rect, xdraw.Over, nil)
		res.Frames = append(res.Frames, cell)
	}

	res.Found = natural
	if natural != expected {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("Detected %[2]d poses instead of the expected %[1]d. Poses may have overlapped or gone missing; regeneration is recommended.", expected, natural))
	}
	return res
}

// bodyExtent returns the minimum size containing 80% of the alpha mass as the "real body" extent.
// It prevents long, extended-limb outliers from overestimating the scale.
func bodyExtent(img *image.NRGBA) (int, int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return 1, 1
	}
	alphaX := make([]float64, w)
	alphaY := make([]float64, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := float64(img.Pix[img.PixOffset(x, y)+3])
			alphaX[x] += a
			alphaY[y] += a
		}
	}
	cutX := cumulativeExtent(alphaX, 0.80)
	cutY := cumulativeExtent(alphaY, 0.80)
	if cutX < 1 {
		cutX = 1
	}
	if cutY < 1 {
		cutY = 1
	}
	return cutX, cutY
}

// cumulativeExtent returns the length of the narrowest contiguous span covering the cumulative mass ratio massFrac.
func cumulativeExtent(mass []float64, massFrac float64) int {
	total := 0.0
	for _, v := range mass {
		total += v
	}
	if total == 0 {
		return 0
	}
	target := total * massFrac
	n := len(mass)
	best := n
	left := 0
	cur := 0.0
	for right := 0; right < n; right++ {
		cur += mass[right]
		for cur >= target {
			if span := right - left + 1; span < best {
				best = span
			}
			cur -= mass[left]
			left++
		}
	}
	return best
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
