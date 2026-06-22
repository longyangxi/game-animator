package sprite

import (
	"image"
	"testing"
)

func TestPresetsCatalog(t *testing.T) {
	if len(Presets) != 100 {
		t.Fatalf("preset count = %d, expected 100", len(Presets))
	}

	seen := map[string]bool{}
	for _, p := range Presets {
		if p.Name == "" || p.Label == "" || p.Category == "" {
			t.Errorf("preset field missing: %+v", p)
		}
		if seen[p.Name] {
			t.Errorf("duplicate preset name: %q", p.Name)
		}
		seen[p.Name] = true

		// the motion hint is central to natural animation quality, so it is required for every keyword
		if len(p.Hint) < 20 {
			t.Errorf("preset %q's motion hint is empty or too short", p.Name)
		}
		if p.Frames < 1 || p.Frames > 10 {
			t.Errorf("preset %q's frame count %d is outside the 1~10 range", p.Name, p.Frames)
		}
		if p.FPS < 1 || p.FPS > 30 {
			t.Errorf("preset %q's FPS %d is outside the 1~30 range", p.Name, p.FPS)
		}
		if p.Action == "" {
			t.Errorf("preset %q's action description missing", p.Name)
		}
	}
}

func TestMotionHintFromCatalog(t *testing.T) {
	// every name in the catalog must be resolvable via MotionHint
	for _, p := range Presets {
		if MotionHint(p.Name) == "" {
			t.Errorf("MotionHint(%q) is empty", p.Name)
		}
	}
	// verify case/whitespace normalization
	if MotionHint("  IDLE  ") == "" {
		t.Error("MotionHint normalization failed")
	}
	// an unregistered name returns an empty string
	if MotionHint("nonexistent-state-xyz") != "" {
		t.Error("an unregistered state must return an empty hint")
	}
}

func TestMotionHintStripsDirectionSuffix(t *testing.T) {
	// state names in an 8-direction set must find the base hint even with a direction suffix
	base := MotionHint("attack")
	if base == "" {
		t.Fatal("attack hint is empty")
	}
	cases := []string{
		"attack-south", "attack-north", "attack-east", "attack-west",
		"attack-south-east", "attack-north-east", "attack-south-west", "attack-north-west",
	}
	for _, name := range cases {
		if got := MotionHint(name); got != base {
			t.Errorf("MotionHint(%q) = %q, should not differ from the attack base hint", name, truncate(got))
		}
	}
	// verify a compound direction is not incorrectly truncated to a single direction
	if stripDirectionSuffix("attack-south-east") != "attack" {
		t.Errorf("compound direction suffix removal failed: %q", stripDirectionSuffix("attack-south-east"))
	}
}

func TestMotionPresence(t *testing.T) {
	mk := func(fill uint8) *image.NRGBA {
		im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for i := 0; i < len(im.Pix); i += 4 {
			im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = fill, fill, fill, 255
		}
		return im
	}
	// two identical frames → motion 0
	if m := MotionPresence([]*image.NRGBA{mk(100), mk(100)}); m != 0 {
		t.Errorf("identical-frame motion = %v, expected 0", m)
	}
	// full black→white change → close to 1
	if m := MotionPresence([]*image.NRGBA{mk(0), mk(255)}); m < 0.7 {
		t.Errorf("large-change motion = %v, should be high", m)
	}
	// a single frame → 0
	if m := MotionPresence([]*image.NRGBA{mk(50)}); m != 0 {
		t.Errorf("single frame = %v, expected 0", m)
	}
}

func truncate(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}

func TestListPresetsHidesHint(t *testing.T) {
	out := ListPresets()
	if len(out) != len(Presets) {
		t.Fatalf("ListPresets length mismatch")
	}
	// modifying the returned slice must not affect the original (it should be a copy)
	if len(out) > 0 {
		out[0].Label = "MUTATED"
		if Presets[0].Label == "MUTATED" {
			t.Error("ListPresets exposed the original catalog (it should be a copy)")
		}
	}
}
