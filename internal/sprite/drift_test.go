package sprite

import (
	"image"
	"strings"
	"testing"
)

// makeCharFrame creates a dummy character frame with the given color composition.
func makeCharFrame(body, hair rgb) *image.NRGBA {
	f := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	set := func(x, y int, c rgb) {
		i := f.PixOffset(x, y)
		f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3] = c.r, c.g, c.b, 255
	}
	for y := 8; y < 56; y++ {
		for x := 16; x < 48; x++ {
			if y < 20 {
				set(x, y, hair)
			} else {
				set(x, y, body)
			}
		}
	}
	return f
}

func TestInspectDriftConsistent(t *testing.T) {
	body, hair := rgb{60, 120, 200}, rgb{180, 140, 40}
	frames := []*image.NRGBA{
		makeCharFrame(body, hair),
		makeCharFrame(body, hair),
		makeCharFrame(body, hair),
		makeCharFrame(body, hair),
	}
	res := InspectFrames(frames, [3]uint8{255, 0, 255}, nil)
	for _, e := range res.Errors {
		if strings.Contains(e, "color composition") {
			t.Errorf("consistent frames flagged as drift: %s", e)
		}
	}
	for _, rep := range res.Reports {
		if rep.PaletteSim < 0.95 {
			t.Errorf("frame %d sim=%.2f, want ~1.0", rep.Index, rep.PaletteSim)
		}
	}
}

func TestInspectDriftDetected(t *testing.T) {
	body, hair := rgb{60, 120, 200}, rgb{180, 140, 40}
	frames := []*image.NRGBA{
		makeCharFrame(body, hair),
		makeCharFrame(body, hair),
		// character identity change: completely different color composition
		makeCharFrame(rgb{20, 200, 60}, rgb{240, 30, 30}),
		makeCharFrame(body, hair),
	}
	res := InspectFrames(frames, [3]uint8{255, 0, 255}, nil)
	found := false
	for _, e := range res.Errors {
		if strings.Contains(e, "Frame 3") && strings.Contains(e, "color composition") {
			found = true
		}
	}
	if !found {
		t.Errorf("drifted frame not detected as error. errors=%v warnings=%v", res.Errors, res.Warnings)
	}
	hintFound := false
	for _, h := range res.RetryHints {
		if strings.Contains(h, "identity") {
			hintFound = true
		}
	}
	if !hintFound {
		t.Error("identity retry hint missing")
	}
}

// TestInspectBaseDrift verifies that the check against the base catches the case where all
// frames drift together and leave-one-out does not catch it.
func TestInspectBaseDrift(t *testing.T) {
	base := makeCharFrame(rgb{60, 120, 200}, rgb{180, 140, 40})
	// the frames are consistent with each other but a completely different color composition from the base
	other, otherHair := rgb{20, 200, 60}, rgb{240, 30, 30}
	frames := []*image.NRGBA{
		makeCharFrame(other, otherHair),
		makeCharFrame(other, otherHair),
		makeCharFrame(other, otherHair),
	}
	res := InspectFrames(frames, [3]uint8{255, 0, 255}, base)
	found := false
	for _, e := range res.Errors {
		if strings.Contains(e, "base character") {
			found = true
		}
	}
	if !found {
		t.Errorf("batch drift not detected. errors=%v warnings=%v", res.Errors, res.Warnings)
	}

	// the same character as the base should produce no error
	same := []*image.NRGBA{
		makeCharFrame(rgb{60, 120, 200}, rgb{180, 140, 40}),
		makeCharFrame(rgb{60, 120, 200}, rgb{180, 140, 40}),
		makeCharFrame(rgb{60, 120, 200}, rgb{180, 140, 40}),
	}
	res = InspectFrames(same, [3]uint8{255, 0, 255}, base)
	for _, e := range res.Errors {
		if strings.Contains(e, "base character") {
			t.Errorf("identical character falsely flagged as base drift: %s", e)
		}
	}

	// a base with no transparent region (e.g. a photo) should skip the check
	opaque := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := 3; i < len(opaque.Pix); i += 4 {
		opaque.Pix[i] = 255 // fully opaque (black background)
	}
	res = InspectFrames(frames, [3]uint8{255, 0, 255}, opaque)
	for _, e := range res.Errors {
		if strings.Contains(e, "base character") {
			t.Errorf("check ran on an opaque base: %s", e)
		}
	}
}
