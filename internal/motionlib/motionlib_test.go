package motionlib

import (
	"bytes"
	"image"
	"testing"

	_ "image/png"
)

func TestTemplateCovered(t *testing.T) {
	for _, name := range []string{"attack", "walk", "run", "cast", "idle", "hurt", "death", "jump", "cheer", "knockback", "block"} {
		raw, ok := Template(name)
		if !ok {
			t.Fatalf("Template(%q) not found", name)
		}
		if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
			t.Errorf("Template(%q) is not a valid PNG: %v", name, err)
		}
	}
}

func TestTemplateDirectionSuffix(t *testing.T) {
	raw, ok := Template("attack-south")
	if !ok || len(raw) == 0 {
		t.Fatalf("Template(\"attack-south\") should resolve to the attack template")
	}
}

func TestTemplateUnknown(t *testing.T) {
	if _, ok := Template("does-not-exist"); ok {
		t.Errorf("Template(unknown) should return ok=false")
	}
	// "taunt" is a real preset but NOT covered by the library.
	if _, ok := Template("taunt"); ok {
		t.Errorf("Template(uncovered) should return ok=false")
	}
}

func TestCovered(t *testing.T) {
	if got := len(Covered()); got != 11 {
		t.Errorf("Covered() len = %d, want 11", got)
	}
}
