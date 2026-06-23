package sprite

import (
	"image"
	"testing"
)

// dominantR returns the most common R value among opaque pixels (the box's fill color).
func dominantR(img *image.NRGBA) int {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	counts := map[uint8]int{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i+3] > alphaThreshold {
				counts[img.Pix[i]]++
			}
		}
	}
	best, bestN := -1, 0
	for r, n := range counts {
		if n > bestN {
			best, bestN = int(r), n
		}
	}
	return best
}

// A 2×3 grid of distinctly-colored poses (with clear row + column gutters) must slice into 6
// frames in row-major order, each carrying its own pose's color (no cross-row/col contamination).
func TestExtractGrid2x3(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 600, 400))
	// Per-pose fill colors; R encodes the row-major index so we can verify ordering.
	rcolor := []uint8{20, 60, 100, 140, 180, 220}
	const cw, ch = 200, 200
	for i := 0; i < 6; i++ {
		r, c := i/3, i%3
		x0, y0 := c*cw+40, r*ch+40
		fillBox(strip, x0, y0, x0+120, y0+120, rcolor[i], 80, 80)
	}

	res := ExtractFrames(strip, 6, 128, 128, 8)
	if len(res.Frames) != 6 {
		t.Fatalf("frames=%d want 6; warnings=%v", len(res.Frames), res.Warnings)
	}
	for i, f := range res.Frames {
		got := dominantR(f)
		if abs(got-int(rcolor[i])) > 12 {
			t.Errorf("frame %d: dominant R=%d, want ~%d (row-major order broken or cross-contamination)", i, got, rcolor[i])
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
