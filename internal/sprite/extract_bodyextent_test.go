package sprite

import (
	"image"
	"testing"
)

// maxOpaqueColHeight returns the height of the tallest opaque column in the frame.
func maxOpaqueColHeight(f *image.NRGBA) int {
	w, h := f.Rect.Dx(), f.Rect.Dy()
	best := 0
	for x := 0; x < w; x++ {
		top, bottom := -1, -1
		for y := 0; y < h; y++ {
			if f.Pix[f.PixOffset(x, y)+3] > alphaThreshold {
				if top < 0 {
					top = y
				}
				bottom = y
			}
		}
		if top >= 0 && bottom-top+1 > best {
			best = bottom - top + 1
		}
	}
	return best
}

// TestExtractBodyExtentScale verifies that one frame's long, thin extended limb (a horizontal
// outlier) does not overestimate the whole strip's scale and shrink the normal frames' bodies too.
//
// Cell 100×100, margin 8 → available 84×84. A normal torso is 60×60.
// Only one frame has a thin arm extending horizontally out to a width of 200.
//   - bbox-based (old): maxW=200 → scale≈84/200≈0.42 → normal torso shrinks to ~25px.
//   - bodyExtent (new): 80% of the mass is concentrated in the 60px body so width≈60 → scale=1 → torso stays ~60px.
func TestExtractBodyExtentScale(t *testing.T) {
	// 1000px wide with four 250px slots. Each slot is separated by a clear magenta gutter.
	strip := image.NewNRGBA(image.Rect(0, 0, 1000, 100))
	// 3 normal frames: 60×60 torso (centered in each slot)
	fillBox(strip, 95, 20, 154, 79, 200, 100, 50)
	fillBox(strip, 595, 20, 654, 79, 200, 100, 50)
	fillBox(strip, 845, 20, 904, 79, 200, 100, 50)
	// outlier frame (slot 2): 60×60 torso + a thin (4px) arm extending right within the slot.
	// The bbox width is 145, but the body (80% of mass) is ~56px.
	fillBox(strip, 345, 20, 404, 79, 200, 100, 50)
	fillBox(strip, 405, 47, 490, 50, 200, 100, 50)

	res := ExtractFrames(strip, 4, 100, 100, 8)
	if res.Found != 4 {
		t.Fatalf("wrong frame count: %d (%v)", res.Found, res.Warnings)
	}

	// The body height of a normal frame (no arm) must not be shrunk by the outlier.
	// bbox-based gives ~25px, bodyExtent gives ~60px. Distinguish with a 45px threshold.
	for _, i := range []int{0, 2, 3} {
		hgt := maxOpaqueColHeight(res.Frames[i])
		if hgt < 45 {
			t.Fatalf("frame %d body shrunk by the outlier: height=%d (suspected bbox-scale regression)", i+1, hgt)
		}
	}
}
