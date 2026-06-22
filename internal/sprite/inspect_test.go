package sprite

import (
	"image"
	"strings"
	"testing"
)

// fillRect fills a rectangle for testing.
func fillRect(img *image.NRGBA, x0, y0, x1, y1 int, r, g, b uint8) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, 255
		}
	}
}

func TestInspectFramesClean(t *testing.T) {
	key := [3]uint8{255, 0, 255}
	var frames []*image.NRGBA
	for i := 0; i < 4; i++ {
		f := image.NewNRGBA(image.Rect(0, 0, 128, 128))
		fillRect(f, 30, 30, 100, 110, 40, 80, 200) // blue character block
		frames = append(frames, f)
	}
	res := InspectFrames(frames, key, nil)
	if !res.Ok() {
		t.Fatalf("error reported on valid frames: %v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warning reported on valid frames: %v", res.Warnings)
	}
}

func TestInspectFramesKeyResidue(t *testing.T) {
	key := [3]uint8{255, 0, 255}
	f := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	fillRect(f, 30, 30, 100, 110, 40, 80, 200)
	fillRect(f, 40, 40, 60, 60, 250, 30, 250) // 400px of magenta residue
	res := InspectFrames([]*image.NRGBA{f}, key, nil)
	if res.Ok() {
		t.Fatal("failed to detect magenta residue")
	}
	joined := strings.Join(res.RetryHints, " ")
	if !strings.Contains(joined, "magenta") {
		t.Fatalf("magenta correction hint missing: %v", res.RetryHints)
	}
}

func TestInspectFramesEdgeAndEmpty(t *testing.T) {
	key := [3]uint8{255, 0, 255}
	// frame touching the edge
	edge := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	fillRect(edge, 0, 30, 70, 110, 40, 80, 200) // starts at x=0 → cut off
	// empty frame
	empty := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	fillRect(empty, 60, 60, 65, 65, 40, 80, 200) // only 25px

	res := InspectFrames([]*image.NRGBA{edge, empty}, key, nil)
	if res.Ok() {
		t.Fatal("failed to detect empty frame as error")
	}
	foundEdge := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "edge") {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatalf("edge cut-off warning missing: %v", res.Warnings)
	}
}

func TestInspectFramesSizeOutlier(t *testing.T) {
	key := [3]uint8{255, 0, 255}
	var frames []*image.NRGBA
	for i := 0; i < 3; i++ {
		f := image.NewNRGBA(image.Rect(0, 0, 128, 128))
		fillRect(f, 20, 20, 110, 110, 40, 80, 200) // 8100px
		frames = append(frames, f)
	}
	tiny := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	fillRect(tiny, 50, 50, 80, 80, 40, 80, 200) // 900px ≈ 0.11×
	frames = append(frames, tiny)

	res := InspectFrames(frames, key, nil)
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "too small") {
			found = true
		}
	}
	if !found {
		t.Fatalf("size outlier warning missing: %v", res.Warnings)
	}
}
