package sprite

import (
	"image"
	"strings"
	"testing"
)

func TestDirectionsMetadata(t *testing.T) {
	if len(Directions) != 8 {
		t.Fatalf("expected 8 directions, got %d", len(Directions))
	}
	gen, mirrored := 0, 0
	seen := map[string]bool{}
	grid := map[[2]int]bool{}
	for _, d := range Directions {
		if seen[d.Key] {
			t.Errorf("duplicate direction key: %s", d.Key)
		}
		seen[d.Key] = true
		if d.Row < 0 || d.Row > 2 || d.Col < 0 || d.Col > 2 {
			t.Errorf("%s: grid coordinate out of range (%d,%d)", d.Key, d.Row, d.Col)
		}
		if grid[[2]int{d.Row, d.Col}] {
			t.Errorf("%s: duplicate grid coordinate (%d,%d)", d.Key, d.Row, d.Col)
		}
		grid[[2]int{d.Row, d.Col}] = true
		if d.MirrorOf == "" {
			gen++
			if FacingPromptSection(d.Key, "") == "" {
				t.Errorf("AI-generated direction %s has no prompt instruction", d.Key)
			}
		} else {
			mirrored++
			src, ok := DirectionByKey(d.MirrorOf)
			if !ok {
				t.Errorf("%s: mirror source %s does not exist", d.Key, d.MirrorOf)
			} else if src.MirrorOf != "" {
				t.Errorf("%s: mirror source %s is itself a mirror direction", d.Key, d.MirrorOf)
			}
		}
	}
	if gen != 5 || mirrored != 3 {
		t.Errorf("expected 5 AI-generated + 3 mirrored, got gen=%d mirrored=%d", gen, mirrored)
	}
	if len(GeneratedDirections) != 5 || GeneratedDirections[0] != "south" {
		t.Errorf("GeneratedDirections must be 5 with south first: %v", GeneratedDirections)
	}
}

func TestIsBackFacing(t *testing.T) {
	for _, k := range []string{"north", "north-east", "north-west"} {
		if !IsBackFacing(k) {
			t.Errorf("%s should be back-facing", k)
		}
	}
	for _, k := range []string{"", "south", "east", "west", "south-east", "south-west"} {
		if IsBackFacing(k) {
			t.Errorf("%s should not be back-facing", k)
		}
	}
}

func TestFacingPromptSection(t *testing.T) {
	if FacingPromptSection("", "") != "" {
		t.Error("an empty direction must give an empty instruction")
	}
	if FacingPromptSection("west", "") != "" {
		t.Error("a mirror direction (west) must have no AI prompt")
	}
	sec := FacingPromptSection("north", "")
	if !strings.Contains(sec, "back view") || !strings.Contains(sec, "Facing direction lock") {
		t.Errorf("the north instruction lacks the back view lock: %q", sec)
	}
}

func TestBuildStripPromptIncludesFacing(t *testing.T) {
	spec := StateSpec{Name: "walk", Frames: 6, FPS: 10, Loop: true, Action: "walking", Facing: "east"}
	p := BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "")
	if !strings.Contains(p, "Facing direction lock") || !strings.Contains(p, "right-side profile") {
		t.Error("the strip prompt must include the direction lock section")
	}
	spec.Facing = ""
	p = BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "")
	if strings.Contains(p, "Facing direction lock") {
		t.Error("when no direction is specified there must be no direction lock section")
	}
}

func TestMirrorNRGBA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	set := func(x, y int, r uint8) {
		i := img.PixOffset(x, y)
		img.Pix[i], img.Pix[i+3] = r, 255
	}
	set(0, 0, 10)
	set(1, 0, 20)
	set(2, 0, 30)
	set(0, 1, 40)

	m := MirrorNRGBA(img)
	get := func(x, y int) uint8 { return m.Pix[m.PixOffset(x, y)] }
	if get(2, 0) != 10 || get(1, 0) != 20 || get(0, 0) != 30 || get(2, 1) != 40 {
		t.Error("horizontal flip result is incorrect")
	}
	// the original is unchanged
	if img.Pix[img.PixOffset(0, 0)] != 10 {
		t.Error("the original image was modified")
	}
	// double flip = original
	mm := MirrorNRGBA(m)
	for i := range img.Pix {
		if mm.Pix[i] != img.Pix[i] {
			t.Fatal("double flip differs from the original")
		}
	}
}
