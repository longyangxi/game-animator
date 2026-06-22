package sprite

import (
	"image"
	"math"
	"testing"
)

// fillBox fills an opaque rectangle in an NRGBA image (inclusive of both ends).
func fillBox(img *image.NRGBA, x0, y0, x1, y1 int, r, g, b uint8) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, 255
		}
	}
}

// torsoCenterX finds the center of the column band (torso) with the most vertical pixels in the frame.
func torsoCenterX(f *image.NRGBA, minCount int) float64 {
	w, h := f.Rect.Dx(), f.Rect.Dy()
	first, last := -1, -1
	for x := 0; x < w; x++ {
		cnt := 0
		for y := 0; y < h; y++ {
			if f.Pix[f.PixOffset(x, y)+3] > alphaThreshold {
				cnt++
			}
		}
		if cnt >= minCount {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return float64(first+last) / 2
}

// TestExtractCentroidAnchor verifies that identical poses all align to the cell center, and that a
// frame with an arm extended to one side wobbles the torso less than bbox-center placement does.
func TestExtractCentroidAnchor(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 480, 100))
	// frames 1·3·4: 20px-wide torso only
	fillBox(strip, 30, 20, 49, 79, 200, 100, 50)
	fillBox(strip, 270, 20, 289, 79, 200, 100, 50)
	fillBox(strip, 390, 20, 409, 79, 200, 100, 50)
	// frame 2: same torso + a thin arm extending far to the right (expands the bbox by 40px)
	fillBox(strip, 150, 20, 169, 79, 200, 100, 50)
	fillBox(strip, 170, 30, 209, 33, 200, 100, 50)

	res := ExtractFrames(strip, 4, 100, 100, 8)
	if res.Found != 4 {
		t.Fatalf("wrong frame count: %d (%v)", res.Found, res.Warnings)
	}

	centers := make([]float64, 4)
	for i, f := range res.Frames {
		centers[i] = torsoCenterX(f, 40) // torso columns are 60px, arm columns are 4px
	}
	minC, maxC := centers[0], centers[0]
	for _, c := range centers[1:] {
		minC = math.Min(minC, c)
		maxC = math.Max(maxC, c)
	}
	spread := maxC - minC
	// With bbox-center placement, frame 2's torso shifts ~20px to the left (spread≈20).
	// With a center-of-mass anchor it shifts only by the arm's mass fraction, so spread must be single-digit.
	if spread >= 6 {
		t.Fatalf("excessive torso wobble: spread=%.1f centers=%v", spread, centers)
	}
	// frames without an arm must align exactly to the cell center (50)
	for _, i := range []int{0, 2, 3} {
		if math.Abs(centers[i]-50) > 2 {
			t.Fatalf("frame %d center-alignment failed: %.1f", i+1, centers[i])
		}
	}
}

// TestExtractSlotGuard verifies that a residual blob far from the poses is not merged into a frame.
func TestExtractSlotGuard(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 600, 100))
	fillBox(strip, 20, 20, 79, 79, 200, 100, 50)   // pose 1 (60×60)
	fillBox(strip, 120, 20, 179, 79, 200, 100, 50) // pose 2 (60×60)
	fillBox(strip, 560, 10, 579, 29, 90, 200, 90)  // distant residue (20×20)

	res := ExtractFrames(strip, 2, 256, 256, 16)
	if res.Found != 2 {
		t.Fatalf("wrong frame count: %d", res.Found)
	}
	for i, f := range res.Frames {
		opaque := 0
		for p := 3; p < len(f.Pix); p += 4 {
			if f.Pix[p] > alphaThreshold {
				opaque++
			}
		}
		// each frame must contain only the 60×60 body (4000 if the 400px residue were merged)
		if opaque != 3600 {
			t.Fatalf("frame %d suspected of merging residue: opaque=%d", i+1, opaque)
		}
	}
}
