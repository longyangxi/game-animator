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
	if !strings.Contains(isoStrip, "isometric") || !strings.Contains(isoStrip, "side-on") {
		t.Errorf("iso strip prompt missing the isometric camera lock / anti-side-view markers: %q", isoStrip)
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

	// Preset walk/run carry a {view} placeholder resolved per perspective.
	// Flat -> "side-view" (2D wording unchanged); iso -> "isometric".
	walkSpec := StateSpec{Name: "walk", Frames: 6, FPS: 10, Loop: true, Action: "{view} walking cycle facing right", Facing: ""}
	isoWalk := BuildStripPrompt("a knight", StylePresets["pixel"], walkSpec, "", "iso")
	if strings.Contains(isoWalk, viewPlaceholder) {
		t.Errorf("iso walk strip left an unresolved %s token: %q", viewPlaceholder, isoWalk)
	}
	if strings.Contains(isoWalk, "side-view walking cycle") || !strings.Contains(isoWalk, "isometric walking cycle") {
		t.Errorf("iso walk action should resolve {view} to isometric, not side-view: %q", isoWalk)
	}
	flatWalk := BuildStripPrompt("a knight", StylePresets["pixel"], walkSpec, "", "")
	if strings.Contains(flatWalk, viewPlaceholder) {
		t.Errorf("flat walk strip left an unresolved %s token: %q", viewPlaceholder, flatWalk)
	}
	if !strings.Contains(flatWalk, "side-view walking cycle") {
		t.Errorf("flat walk action must resolve {view} to side-view (2D unchanged): %q", flatWalk)
	}

	// A baked travel direction ("facing right") is dropped once an explicit
	// Facing is chosen (the Facing direction lock owns orientation), but kept
	// when no direction is selected (preserves the 2D default).
	walkEast := walkSpec
	walkEast.Facing = "east"
	if strings.Contains(BuildStripPrompt("a knight", StylePresets["pixel"], walkEast, "", ""), "facing right") {
		t.Error("walk with a chosen Facing must drop the baked 'facing right'")
	}
	if !strings.Contains(flatWalk, "facing right") {
		t.Error(`walk with no Facing ("Not set") must keep its default "facing right"`)
	}

	// ViewToken is the single source of truth for the view word.
	if ViewToken("") != "side-view" || ViewToken("flat") != "side-view" || ViewToken("iso") != "isometric" {
		t.Errorf("ViewToken wrong: flat=%q iso=%q", ViewToken("flat"), ViewToken("iso"))
	}

	// Iso strips the flat-camera phrasing from the directional facings; flat keeps it.
	if strings.Contains(FacingPromptSection("south", "iso"), "at eye level") {
		t.Error(`iso south facing must drop "at eye level"`)
	}
	if !strings.Contains(FacingPromptSection("south", ""), "at eye level") {
		t.Error(`flat south facing must keep "at eye level"`)
	}
	if strings.Contains(FacingPromptSection("east", "iso"), "strictly 2D profile") {
		t.Error(`iso east facing must drop "strictly 2D profile, no perspective rotation"`)
	}
	if !strings.Contains(FacingPromptSection("east", ""), "strictly 2D profile") {
		t.Error(`flat east facing must keep "strictly 2D profile"`)
	}
}
