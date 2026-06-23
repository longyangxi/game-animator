package sprite

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite prompt golden files")

// checkGolden compares got against testdata/<name>.golden, or rewrites it under -update-golden.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run: go test ./internal/sprite -run TestPromptGoldenFlat -update-golden): %v", path, err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden — flat prompt output must stay byte-identical", name)
	}
}

func TestPromptGoldenFlat(t *testing.T) {
	checkGolden(t, "character_flat", BuildCharacterPrompt("a knight", StylePresets["pixel"], ""))

	specEast := StateSpec{Name: "walk", Frames: 6, FPS: 10, Loop: true, Action: "walking", Facing: "east"}
	checkGolden(t, "strip_flat_east", BuildStripPrompt("a knight", StylePresets["pixel"], specEast, "make arms bigger", ""))

	specNoFacing := StateSpec{Name: "attack", Frames: 5, FPS: 12, Loop: false, Action: "melee attack", Facing: ""}
	checkGolden(t, "strip_flat_nofacing", BuildStripPrompt("a knight", StylePresets["pixel"], specNoFacing, "", ""))

	checkGolden(t, "facing_south", FacingPromptSection("south", ""))
	checkGolden(t, "facing_north", FacingPromptSection("north", ""))
}

func TestPromptIso(t *testing.T) {
	flatChar := BuildCharacterPrompt("a knight", StylePresets["pixel"], "")
	isoChar := BuildCharacterPrompt("a knight", StylePresets["pixel"], "iso")
	if isoChar == flatChar {
		t.Fatal("iso character prompt must differ from flat")
	}
	if !strings.Contains(isoChar, "isometric") || !strings.Contains(isoChar, "35 degrees") {
		t.Errorf("iso character prompt missing isometric markers: %q", isoChar)
	}

	spec := StateSpec{Name: "attack", Frames: 5, FPS: 12, Loop: false, Action: "melee attack", Facing: ""}
	flatStrip := BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "")
	isoStrip := BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "iso")
	if isoStrip == flatStrip {
		t.Fatal("iso strip prompt must differ from flat even with no facing")
	}
	if !strings.Contains(isoStrip, "Isometric view lock") {
		t.Errorf("iso strip prompt missing the isometric view lock: %q", isoStrip)
	}

	// "flat" string is treated the same as empty.
	if BuildCharacterPrompt("a knight", StylePresets["pixel"], "flat") != flatChar {
		t.Error(`perspective "flat" must equal empty-string output`)
	}

	// Iso adds a tilt line to a directional facing section.
	if !strings.Contains(FacingPromptSection("south", "iso"), "Isometric tilt") {
		t.Error("iso facing section missing the isometric tilt line")
	}
	if FacingPromptSection("south", "") == FacingPromptSection("south", "iso") {
		t.Error("iso facing section must differ from flat")
	}
}
