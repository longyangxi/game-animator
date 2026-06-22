package sprite

import (
	"image"
	"testing"
)

// TestSegmentCleanGutters verifies that the natural pose count and segmentation are correct
// on a strip with clean gutters (the projection-gutter split path).
func TestSegmentCleanGutters(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 600, 100))
	fillBox(strip, 20, 20, 79, 79, 200, 100, 50)
	fillBox(strip, 220, 20, 279, 79, 200, 100, 50)
	fillBox(strip, 420, 20, 479, 79, 200, 100, 50)

	segs, natural := segmentStrip(strip, 3)
	if natural != 3 {
		t.Fatalf("wrong natural pose count: %d", natural)
	}
	if len(segs) != 3 {
		t.Fatalf("wrong segment count: %d", len(segs))
	}
}

// TestSegmentTouchingDP verifies that when two poses touch via a thin part and become one run,
// prominence peak detection + DP minimum cut separates the two torsos into exactly 2
// (the joined-pose case where the connected-component method fails).
func TestSegmentTouchingDP(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 400, 100))
	// torso A (tall) ── connected by a thin arm (low mass) ── torso B (tall): one run, but
	// the valley between is deep enough to split into two peaks.
	fillBox(strip, 40, 10, 110, 89, 200, 100, 50)  // torso A
	fillBox(strip, 110, 48, 250, 55, 200, 100, 50) // thin connector
	fillBox(strip, 250, 10, 330, 89, 200, 100, 50) // torso B

	res := ExtractFrames(strip, 2, 100, 100, 8)
	if len(res.Frames) != 2 {
		t.Fatalf("failed to separate touching poses: %d frames (expected 2)", len(res.Frames))
	}
	// both frames must have substantial content
	for i, f := range res.Frames {
		opaque := 0
		for p := 3; p < len(f.Pix); p += 4 {
			if f.Pix[p] > alphaThreshold {
				opaque++
			}
		}
		if opaque < 500 {
			t.Fatalf("frame %d has insufficient content: %d", i+1, opaque)
		}
	}
}

// TestChromaMatteYCbCr verifies that a green square on a magenta background is correctly
// separated by chroma-plane matting (green opaque, magenta transparent).
func TestChromaMatteYCbCr(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i+3 < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = 255, 0, 255, 255 // magenta
	}
	fillBox(src, 20, 20, 43, 43, 40, 200, 60) // green square

	out := RemoveBackground(src)
	// center (green) is opaque
	ci := out.PixOffset(32, 32)
	if out.Pix[ci+3] < 200 {
		t.Fatalf("green subject was made transparent: alpha=%d", out.Pix[ci+3])
	}
	// corner (magenta) is transparent
	ei := out.PixOffset(2, 2)
	if out.Pix[ei+3] > alphaThreshold {
		t.Fatalf("magenta background remained: alpha=%d", out.Pix[ei+3])
	}
}
