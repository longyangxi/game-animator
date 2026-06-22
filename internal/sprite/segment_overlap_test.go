package sprite

import (
	"image"
	"testing"
)

// TestSegmentForceExpectedOnOverlap verifies that even when the AI draws overlapping poses
// with no magenta gutter, recovery via DP splitting restores the expected count.
func TestSegmentForceExpectedOnOverlap(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 400, 100))
	fillBox(strip, 40, 20, 140, 79, 200, 100, 50)
	fillBox(strip, 120, 20, 220, 79, 200, 100, 50)
	segs, natural := segmentStrip(strip, 2)
	if natural != 2 {
		t.Fatalf("natural=%d segs=%v", natural, segs)
	}
}
