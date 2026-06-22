package sprite

import (
	"image"
	"sort"
)

// DetectPixelScale estimates the actual pixel block size of AI-generated "fake pixel art".
// It uses the mode of horizontal/vertical same-color run lengths (the unfake.js technique).
// Returns 1 if detection fails or the image is already at native resolution.
func DetectPixelScale(img *image.NRGBA) int {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w < 32 || h < 32 {
		return 1
	}
	maxScale := min(w, h) / 8
	if maxScale > 32 {
		maxScale = 32
	}
	if maxScale < 2 {
		return 1
	}
	hist := make([]int, maxScale+1)

	countRuns := func(get func(a, b int) (uint8, uint8, uint8, uint8), outer, inner int) {
		for o := 0; o < outer; o++ {
			runLen := 1
			pr, pg, pb, pa := get(o, 0)
			for i := 1; i < inner; i++ {
				r, g, b, al := get(o, i)
				same := (al <= alphaThreshold && pa <= alphaThreshold) ||
					(al > alphaThreshold && pa > alphaThreshold && nearRGB(r, g, b, pr, pg, pb))
				if same {
					runLen++
				} else {
					if runLen >= 2 && runLen <= maxScale {
						hist[runLen]++
					}
					runLen = 1
				}
				pr, pg, pb, pa = r, g, b, al
			}
		}
	}
	countRuns(func(y, x int) (uint8, uint8, uint8, uint8) {
		i := img.PixOffset(x, y)
		return img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
	}, h, w)
	countRuns(func(x, y int) (uint8, uint8, uint8, uint8) {
		i := img.PixOffset(x, y)
		return img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
	}, w, h)

	best, bestCount := 1, 0
	for s := 2; s <= maxScale; s++ {
		// short runs are always numerous, so weight by run length
		weighted := hist[s] * s
		if weighted > bestCount {
			best, bestCount = s, weighted
		}
	}
	if best < 2 {
		return 1
	}
	return best
}

// nearRGB is a color comparison that tolerates slight AA noise.
func nearRGB(r1, g1, b1, r2, g2, b2 uint8) bool {
	const tol = 12
	return absInt(int(r1)-int(r2)) <= tol && absInt(int(g1)-int(g2)) <= tol && absInt(int(b1)-int(b2)) <= tol
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Pixelize snaps the image to scale×scale blocks to form a true pixel-art grid.
// It adopts each block's dominant color and keeps the output size identical to the input.
func Pixelize(img *image.NRGBA, scale int) *image.NRGBA {
	if scale < 2 {
		return img
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	type counted struct {
		c rgb
		n int
	}
	for by := 0; by < h; by += scale {
		for bx := 0; bx < w; bx += scale {
			bw, bh := min(scale, w-bx), min(scale, h-by)
			// determine the block's dominant color (alpha majority → most frequent color)
			opaque := 0
			counts := make(map[rgb]int, 8)
			for dy := 0; dy < bh; dy++ {
				for dx := 0; dx < bw; dx++ {
					i := img.PixOffset(bx+dx, by+dy)
					if img.Pix[i+3] <= alphaThreshold {
						continue
					}
					opaque++
					counts[rgb{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}]++
				}
			}
			if opaque*2 < bw*bh {
				continue // block is mostly transparent → empty block
			}
			var dom counted
			for c, n := range counts {
				if n > dom.n {
					dom = counted{c, n}
				}
			}
			for dy := 0; dy < bh; dy++ {
				for dx := 0; dx < bw; dx++ {
					i := out.PixOffset(bx+dx, by+dy)
					out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = dom.c.r, dom.c.g, dom.c.b, 255
				}
			}
		}
	}
	return out
}

// PaletteSizeForStyle returns the recommended palette color count per style (0 disables post-processing).
func PaletteSizeForStyle(styleKey string) int {
	switch styleKey {
	case "retro16":
		return 16
	case "pixel":
		return 32
	default:
		return 0 // chibi/cartoon/custom do not force a pixel grid
	}
}

// PixelPostProcess applies shared-palette quantization + pixel-grid snapping to a state's bundle of frames.
// The frames are modified in-place or replaced.
func PixelPostProcess(frames []*image.NRGBA, paletteSize int) {
	if paletteSize <= 0 || len(frames) == 0 {
		return
	}
	palette := BuildSharedPalette(frames, paletteSize)
	if palette == nil {
		return
	}
	scales := make([]int, 0, len(frames))
	for _, f := range frames {
		ApplyPalette(f, palette)
		scales = append(scales, DetectPixelScale(f))
	}
	// share the median scale for grid consistency across frames
	sorted := append([]int(nil), scales...)
	sort.Ints(sorted)
	scale := sorted[len(sorted)/2]
	if scale < 2 {
		return
	}
	for i, f := range frames {
		frames[i] = Pixelize(f, scale)
	}
}
