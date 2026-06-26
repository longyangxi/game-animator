# Motion-Reference Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global, default-ON Settings toggle that injects a validated pose-template image plus a pose-transfer instruction into animation generation for ~11 common actions, with automatic text-only fallback.

**Architecture:** A new `internal/motionlib` package embeds pre-cropped front-row pose strips keyed by preset name. `GenerateState` in `app.go` appends the matching template to the provider reference images and appends `sprite.PoseTemplateClause(...)` to the prompt, but only when the toggle is on and the action is not a directional set (which keeps its existing same-character `RefStrip`). A `*bool` config field (`Settings.MotionLibrary`) makes "unset" mean ON. The frontend adds one toggle in `SettingsModal.tsx` wired to a new `SetMotionLibrary` Wails method.

**Tech Stack:** Go 1.25, Wails v2, React 18 + TypeScript, Tailwind, Go `embed`.

## Global Constraints

- Go version floor: 1.25 (per `go.mod`).
- Toggle default: **ON** when unset (new users and pre-existing config files).
- When the toggle is OFF, generation output must be byte-identical to current behavior (no extra refs, no prompt changes).
- Directional `RefStrip` (8-direction sets) always takes precedence; the library template is used **only** when `RefStrip` is empty.
- Library templates are **validation-phase placeholders** sourced from `image-cockpit-for-codex-workflows` (`public/samples/*-sheet.png`, license marked `sample`); they are a generation *input* only, never shipped in user output, and must be replaced with our own art before any public release. Document this in `internal/motionlib/README.md`.
- Covered actions (11): `idle, walk, run, attack, hurt, death, cast, jump, cheer, knockback, block`.
- i18n: every user-facing string must be added to all four locale files: `ui.en.ts`, `ui.es.ts`, `ui.ko.ts`, `ui.zh.ts`.
- Commit messages prefixed `Claude-5: ` and end with the `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` trailer.

---

## File Structure

- Create `internal/motionlib/motionlib.go` — embeds templates, exposes `Template`, `Covered`.
- Create `internal/motionlib/motionlib_test.go` — package tests.
- Create `internal/motionlib/templates/*.png` — 11 pre-cropped front-row strips (committed binary assets).
- Create `internal/motionlib/README.md` — provenance + replace-before-release caveat + mapping.
- Create `internal/motionlib/cropfront.go` — `//go:build ignore` generator that downloads sheets and crops front rows (reproducibility; not part of the build).
- Modify `internal/sprite/presets.go` — add exported `BaseStateName`.
- Modify `internal/sprite/prompt.go` — add `PoseTemplateClause`.
- Create `internal/sprite/prompt_pose_test.go` — golden test for `PoseTemplateClause` + `BaseStateName`.
- Modify `cmd/ppposetest/main.go` — call `sprite.PoseTemplateClause` instead of the local copy (DRY).
- Modify `internal/config/config.go` — add `Settings.MotionLibrary *bool` + `MotionLibraryEnabled()`.
- Modify `internal/config/config_test.go` — test the helper + round-trip.
- Modify `app.go` — add `libraryTemplate` helper, wire `GenerateState`, add `SetMotionLibrary`, extend `SettingsInfo` + `GetSettings`.
- Modify `app_test.go` — test `libraryTemplate`.
- Modify `frontend/src/components/SettingsModal.tsx` — toggle row.
- Modify `frontend/src/i18n/ui.{en,es,ko,zh}.ts` — labels.
- Regenerate `frontend/wailsjs/go/main/App.{js,d.ts}` — new `SetMotionLibrary` binding.

---

## Task 1: Shared prompt helpers in `sprite`

**Files:**
- Modify: `internal/sprite/presets.go` (add `BaseStateName` near `stripDirectionSuffix`, ~line 173)
- Modify: `internal/sprite/prompt.go` (add `PoseTemplateClause` at end of file)
- Modify: `cmd/ppposetest/main.go` (replace local `poseTemplateClause` with the sprite call)
- Test: `internal/sprite/prompt_pose_test.go`

**Interfaces:**
- Produces: `sprite.BaseStateName(name string) string` — lowercases, trims, strips a trailing direction suffix (e.g. `"attack-south"` → `"attack"`, `"attack"` → `"attack"`).
- Produces: `sprite.PoseTemplateClause(stateName string, frames int) string` — the pose-transfer instruction block (leading newline, trailing newline).

- [ ] **Step 1: Write the failing test**

Create `internal/sprite/prompt_pose_test.go`:

```go
package sprite

import "strings"

import "testing"

func TestBaseStateName(t *testing.T) {
	cases := map[string]string{
		"attack":            "attack",
		"Attack":            "attack",
		"  attack ":         "attack",
		"attack-south":      "attack",
		"attack-north-east": "attack",
		"walk":              "walk",
	}
	for in, want := range cases {
		if got := BaseStateName(in); got != want {
			t.Errorf("BaseStateName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPoseTemplateClause(t *testing.T) {
	got := PoseTemplateClause("attack", 5)
	for _, want := range []string{
		"Pose template (CRITICAL",
		"Image 1 is the CANONICAL CHARACTER",
		"Image 2 is a POSE TEMPLATE",
		`"attack" action`,
		"EXACTLY 5 poses",
		"Do NOT reproduce any glow, slash-arc, swoosh",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PoseTemplateClause missing %q\n--- got ---\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sprite/ -run 'TestBaseStateName|TestPoseTemplateClause' -v`
Expected: FAIL — `undefined: BaseStateName`, `undefined: PoseTemplateClause`.

- [ ] **Step 3: Add `BaseStateName` to `presets.go`**

Append after `stripDirectionSuffix` (end of `internal/sprite/presets.go`):

```go
// BaseStateName lowercases, trims, and strips a trailing direction suffix from a
// state name, yielding the base preset keyword (e.g. "attack-south" -> "attack").
func BaseStateName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	return stripDirectionSuffix(key)
}
```

- [ ] **Step 4: Add `PoseTemplateClause` to `prompt.go`**

Append to the end of `internal/sprite/prompt.go`:

```go
// PoseTemplateClause instructs the model to treat a SECOND attached image as a
// pose/motion template from a DIFFERENT character: identity comes from image 1,
// only the body choreography comes from image 2, and image 2's effects/colours
// are explicitly discarded. Shared by app.go (motion library) and cmd/ppposetest.
func PoseTemplateClause(stateName string, frames int) string {
	return fmt.Sprintf(`
Pose template (CRITICAL — read carefully): TWO images are attached.
- Image 1 is the CANONICAL CHARACTER. Take ALL identity from it: face, hairstyle, body build, outfit, armor, cape, palette, and the weapon or signature prop. The output MUST be image 1's character.
- Image 2 is a POSE TEMPLATE showing a DIFFERENT character performing the "%s" action across several frames, read left to right. Copy ONLY the body choreography from image 2: stance, limb positions, weight shift, wind-up, strike, and recovery arc. Re-pose OUR character (image 1) through those same key poses and motion timing.
- Do NOT copy image 2's character, colours, outfit, weapon shape, or its background. Do NOT reproduce any glow, slash-arc, swoosh, streak, spark, or motion effect drawn in image 2 — render only OUR character's solid body in those poses on the clean keying background.
- Distribute the template's motion across our EXACTLY %d poses (start/wind-up, peak strike, settle).
`, stateName, frames)
}
```

`prompt.go` already imports `fmt` and `strings`, so no import changes are needed.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/sprite/ -run 'TestBaseStateName|TestPoseTemplateClause' -v`
Expected: PASS (both tests).

- [ ] **Step 6: DRY — point the experiment CLI at the shared clause**

In `cmd/ppposetest/main.go`: delete the local `poseTemplateClause` function and replace its two call sites (`promptA` and `promptB` construction) — change `+ poseTemplateClause(stateName, pre.Frames)` to `+ sprite.PoseTemplateClause(stateName, pre.Frames)`.

- [ ] **Step 7: Verify the whole module still builds**

Run: `go build ./...`
Expected: success, no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/sprite/presets.go internal/sprite/prompt.go internal/sprite/prompt_pose_test.go cmd/ppposetest/main.go
git commit -m "$(printf 'Claude-5: Add sprite.PoseTemplateClause + BaseStateName helpers [kanban-LmE2cl]\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 2: `internal/motionlib` embedded library

**Files:**
- Create: `internal/motionlib/cropfront.go` (generator, `//go:build ignore`)
- Create: `internal/motionlib/templates/*.png` (11 cropped front-row strips)
- Create: `internal/motionlib/motionlib.go`
- Create: `internal/motionlib/README.md`
- Test: `internal/motionlib/motionlib_test.go`

**Interfaces:**
- Consumes: `sprite.BaseStateName` (Task 1).
- Produces: `motionlib.Template(stateName string) ([]byte, bool)` — returns the embedded PNG strip for the action (after base-name resolution), or `(nil, false)`.
- Produces: `motionlib.Covered() []string` — sorted list of covered preset names.

- [ ] **Step 1: Write the front-row crop generator**

Create `internal/motionlib/cropfront.go`:

```go
//go:build ignore

// cropfront downloads image-cockpit official animation sheets and writes the
// FRONT row (row 0 of 5) of each to internal/motionlib/templates/<preset>.png.
// Run from the repo root: `go run ./internal/motionlib/cropfront.go`
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	_ "image/png"
)

const base = "https://raw.githubusercontent.com/dreiachse-cyber/image-cockpit-for-codex-workflows/main/public/samples/"

// sheet filename (without -sheet.png) -> our preset name
var mapping = map[string]string{
	"idle-breathing":  "idle",
	"walk-cycle":      "walk",
	"run-cycle":       "run",
	"basic-attack":    "attack",
	"hurt-reaction":   "hurt",
	"death-downed":    "death",
	"spell-cast":      "cast",
	"jump-hop":        "jump",
	"victory-cheer":   "cheer",
	"knockback":       "knockback",
	"guard-block":     "block",
}

func main() {
	out := filepath.Join("internal", "motionlib", "templates")
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	for src, preset := range mapping {
		url := base + src + "-sheet.png"
		resp, err := http.Get(url)
		if err != nil {
			panic(err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			panic(err)
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			panic(fmt.Errorf("%s: %w", src, err))
		}
		b := img.Bounds()
		rowH := b.Dy() / 5 // sheets are 8 columns x 5 direction rows
		sub := image.NewNRGBA(image.Rect(0, 0, b.Dx(), rowH))
		for y := 0; y < rowH; y++ {
			for x := 0; x < b.Dx(); x++ {
				sub.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		f, err := os.Create(filepath.Join(out, preset+".png"))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, sub); err != nil {
			panic(err)
		}
		f.Close()
		fmt.Printf("wrote %s.png (%dx%d)\n", preset, b.Dx(), rowH)
	}
}
```

- [ ] **Step 2: Generate the template assets**

Run: `go run ./internal/motionlib/cropfront.go`
Expected: 11 lines `wrote <preset>.png (2048x256)`, and `internal/motionlib/templates/` contains 11 PNGs: `idle.png walk.png run.png attack.png hurt.png death.png cast.png jump.png cheer.png knockback.png block.png`.

Verify: `ls internal/motionlib/templates/ | wc -l` → `11`.

- [ ] **Step 3: Write the failing package test**

Create `internal/motionlib/motionlib_test.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/motionlib/ -v`
Expected: FAIL — `undefined: Template`, `undefined: Covered`.

- [ ] **Step 5: Write the package implementation**

Create `internal/motionlib/motionlib.go`:

```go
// Package motionlib provides validated pose-template sprite strips used as a
// generation reference image to drive animation motion quality. Templates are
// front-row crops of borrowed sheets (see README.md) and are an internal
// generation input only — never shipped in user output.
package motionlib

import (
	"embed"
	"sort"

	"perfectpixel/internal/sprite"
)

//go:embed templates/*.png
var templates embed.FS

// Template returns the embedded pose strip for a state name (its base keyword
// after stripping any direction suffix), or (nil, false) if uncovered.
func Template(stateName string) ([]byte, bool) {
	base := sprite.BaseStateName(stateName)
	data, err := templates.ReadFile("templates/" + base + ".png")
	if err != nil {
		return nil, false
	}
	return data, true
}

// Covered returns the sorted list of preset names that have a template.
func Covered() []string {
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > 4 && name[len(name)-4:] == ".png" {
			out = append(out, name[:len(name)-4])
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/motionlib/ -v`
Expected: PASS (all four tests).

- [ ] **Step 7: Write the provenance README**

Create `internal/motionlib/README.md`:

```markdown
# motionlib — pose-template library

`templates/<preset>.png` are 8-frame **front-row** strips used as a generation
reference image to guide animation motion (see `sprite.PoseTemplateClause`).

## ⚠️ Validation-phase placeholders

These strips are cropped from `dreiachse-cyber/image-cockpit-for-codex-workflows`
(`public/samples/*-sheet.png`, license marked `sample`). They are used **only**
as an internal generation input and never appear in user output. **Replace them
with our own validated art before any public release.**

Regenerate with: `go run ./internal/motionlib/cropfront.go`

## Source sheet -> preset mapping

| image-cockpit sheet | preset |
|---|---|
| idle-breathing | idle |
| walk-cycle | walk |
| run-cycle | run |
| basic-attack | attack |
| hurt-reaction | hurt |
| death-downed | death |
| spell-cast | cast |
| jump-hop | jump |
| victory-cheer | cheer |
| knockback | knockback |
| guard-block | block |
```

- [ ] **Step 8: Commit**

```bash
git add internal/motionlib/
git commit -m "$(printf 'Claude-5: Add motionlib embedded pose-template library [kanban-LmE2cl]\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 3: Config flag

**Files:**
- Modify: `internal/config/config.go` (Settings struct ~line 19, add helper)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Settings.MotionLibrary *bool` (json `motionLibrary,omitempty`).
- Produces: `(s *Settings) MotionLibraryEnabled() bool` — `nil` → true, else `*ptr`.

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestMotionLibraryEnabled(t *testing.T) {
	var s Settings
	if !s.MotionLibraryEnabled() {
		t.Errorf("unset MotionLibrary should default to enabled (true)")
	}
	off := false
	s.MotionLibrary = &off
	if s.MotionLibraryEnabled() {
		t.Errorf("explicit false should be disabled")
	}
	on := true
	s.MotionLibrary = &on
	if !s.MotionLibraryEnabled() {
		t.Errorf("explicit true should be enabled")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestMotionLibraryEnabled -v`
Expected: FAIL — `s.MotionLibrary undefined` / `s.MotionLibraryEnabled undefined`.

- [ ] **Step 3: Add the field and helper**

In `internal/config/config.go`, add to the `Settings` struct (after the provider fields, before the legacy fields ~line 28):

```go
	MotionLibrary *bool `json:"motionLibrary,omitempty"` // nil = default ON
```

Add the helper after the `Cfg` method (~line 34):

```go
// MotionLibraryEnabled reports whether the motion-reference library is on.
// Unset (nil) defaults to true so the feature is on for new and existing users.
func (s *Settings) MotionLibraryEnabled() bool {
	return s.MotionLibrary == nil || *s.MotionLibrary
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestMotionLibraryEnabled -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "$(printf 'Claude-5: Add MotionLibrary config flag (default ON) [kanban-LmE2cl]\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 4: Backend wiring in `app.go`

**Files:**
- Modify: `app.go` (imports; `GenerateState` ~lines 398-421; `SettingsInfo` ~line 113; `GetSettings` ~line 126; new `SetMotionLibrary` after `SetProvider` ~line 189; new `libraryTemplate` helper)
- Test: `app_test.go`

**Interfaces:**
- Consumes: `motionlib.Template` (Task 2), `sprite.PoseTemplateClause` (Task 1), `config.Settings.MotionLibraryEnabled` (Task 3).
- Produces: `libraryTemplate(stateName string, refStripPresent, enabled bool) ([]byte, bool)` — returns the template bytes to append + whether the pose clause should be added.
- Produces: `(a *App) SetMotionLibrary(enabled bool) error`.
- Produces: `SettingsInfo.MotionLibrary bool` field.

- [ ] **Step 1: Write the failing test for the decision helper**

Add to `app_test.go`:

```go
func TestLibraryTemplate(t *testing.T) {
	// enabled, covered action, no RefStrip -> template returned, clause on
	tpl, use := libraryTemplate("attack", false, true)
	if !use || len(tpl) == 0 {
		t.Fatalf("attack/enabled/no-refstrip should use the library template")
	}
	// disabled -> never
	if _, use := libraryTemplate("attack", false, false); use {
		t.Errorf("disabled toggle must not use the library")
	}
	// RefStrip present (directional set) -> never
	if _, use := libraryTemplate("attack", true, true); use {
		t.Errorf("RefStrip precedence: must not use the library")
	}
	// uncovered action -> never
	if _, use := libraryTemplate("taunt", false, true); use {
		t.Errorf("uncovered action must not use the library")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestLibraryTemplate -v`
Expected: FAIL — `undefined: libraryTemplate`.

- [ ] **Step 3: Add the `libraryTemplate` helper and import**

Add `"perfectpixel/internal/motionlib"` to the import block in `app.go`.

Add the helper (place it just above `GenerateState`):

```go
// libraryTemplate decides whether a motion-library pose template applies to this
// generation. It returns the template PNG bytes and whether the pose-transfer
// clause should be appended. The directional RefStrip always wins, so the
// library is used only when enabled, no RefStrip is set, and the action is covered.
func libraryTemplate(stateName string, refStripPresent, enabled bool) ([]byte, bool) {
	if !enabled || refStripPresent {
		return nil, false
	}
	return motionlib.Template(stateName)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test . -run TestLibraryTemplate -v`
Expected: PASS.

- [ ] **Step 5: Wire `GenerateState` to use the helper**

In `app.go`, locate the reference-image block (currently ~lines 398-404):

```go
	// generation reference images: base character + (optional) front strip
	refs := [][]byte{baseRaw}
	if strings.TrimSpace(args.RefStrip) != "" {
		if refRaw, err := decodeDataURL(args.RefStrip); err == nil {
			refs = append(refs, refRaw)
		}
	}
```

Replace it with:

```go
	// generation reference images: base character + (optional) front strip
	refs := [][]byte{baseRaw}
	refStripPresent := strings.TrimSpace(args.RefStrip) != ""
	if refStripPresent {
		if refRaw, err := decodeDataURL(args.RefStrip); err == nil {
			refs = append(refs, refRaw)
		}
	}
	// motion-reference library: when enabled and not a directional set, append a
	// validated pose template so the model copies its choreography onto our character.
	motionTemplate := false
	if tpl, use := libraryTemplate(args.State.Name, refStripPresent, config.Load().MotionLibraryEnabled()); use {
		refs = append(refs, tpl)
		motionTemplate = true
	}
```

Then locate the prompt assembly in the attempt loop (currently ~lines 418-421):

```go
		prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, args.Perspective)
		if len(refs) > 1 {
			prompt += "\nMotion reference: the second attached image is the FRONT-view animation strip of this same character performing this exact action. Reproduce the same motion timing and pose phases frame by frame, but viewed from the required facing direction above.\n"
		}
```

Replace it with:

```go
		prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, args.Perspective)
		if motionTemplate {
			prompt += sprite.PoseTemplateClause(args.State.Name, args.State.Frames)
		} else if len(refs) > 1 {
			prompt += "\nMotion reference: the second attached image is the FRONT-view animation strip of this same character performing this exact action. Reproduce the same motion timing and pose phases frame by frame, but viewed from the required facing direction above.\n"
		}
```

(`config` is already imported in `app.go`; verify with `grep '"perfectpixel/internal/config"' app.go`.)

- [ ] **Step 6: Extend `SettingsInfo` and `GetSettings`**

In `app.go`, add a field to `SettingsInfo` (~line 113):

```go
	MotionLibrary bool `json:"motionLibrary"`
```

In `GetSettings` (~line 128), set it when building `info`:

```go
	info := SettingsInfo{Provider: s.Provider, Providers: map[string]ProviderInfo{}, MotionLibrary: s.MotionLibraryEnabled()}
```

- [ ] **Step 7: Add the `SetMotionLibrary` Wails method**

Add after `SetProvider` (~line 189) in `app.go`:

```go
// SetMotionLibrary turns the motion-reference library on or off.
func (a *App) SetMotionLibrary(enabled bool) error {
	s := config.Load()
	s.MotionLibrary = &enabled
	return config.Save(s)
}
```

- [ ] **Step 8: Run backend tests + build**

Run: `go test . ./internal/... && go build ./...`
Expected: all PASS, build succeeds.

- [ ] **Step 9: Commit**

```bash
git add app.go app_test.go
git commit -m "$(printf 'Claude-5: Wire motion-library pose template into GenerateState [kanban-LmE2cl]\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Task 5: Frontend toggle + i18n + bindings

**Files:**
- Modify: `frontend/src/i18n/ui.en.ts`, `ui.es.ts`, `ui.ko.ts`, `ui.zh.ts`
- Modify: `frontend/src/components/SettingsModal.tsx`
- Regenerate: `frontend/wailsjs/go/main/App.js`, `App.d.ts`

**Interfaces:**
- Consumes: `GetSettings().motionLibrary` (bool), `SetMotionLibrary(enabled: boolean)` Wails binding (Task 4).

- [ ] **Step 1: Regenerate Wails bindings for the new method**

Run: `wails generate module`
Expected: `frontend/wailsjs/go/main/App.js` and `App.d.ts` now contain `SetMotionLibrary`. Verify: `grep -r SetMotionLibrary frontend/wailsjs/`.

(If `wails generate module` is unavailable in the environment, hand-add to `frontend/wailsjs/go/main/App.js`:
```js
export function SetMotionLibrary(arg1) { return window['go']['main']['App']['SetMotionLibrary'](arg1); }
```
and to `App.d.ts`:
```ts
export function SetMotionLibrary(arg1:boolean):Promise<void>;
```)

- [ ] **Step 2: Add i18n labels (all four locales)**

Open `frontend/src/i18n/ui.en.ts` and find the settings key group used by `SettingsModal.tsx` (look for existing keys like `settingsProvider` / `settingsTitle`). Add two keys consistent with the file's existing style, e.g.:

`ui.en.ts`:
```ts
  settingsMotionLibrary: 'Use motion reference (beta)',
  settingsMotionLibraryHint: 'Guide animations with a validated pose template for common actions. Turn off to use text prompts only.',
```
`ui.zh.ts`:
```ts
  settingsMotionLibrary: '使用动作参考图（实验）',
  settingsMotionLibraryHint: '用验证过的姿势模板引导常用动作的动画。关闭则只用文字提示。',
```
`ui.ko.ts`:
```ts
  settingsMotionLibrary: '모션 레퍼런스 사용 (베타)',
  settingsMotionLibraryHint: '검증된 포즈 템플릿으로 일반 동작 애니메이션을 안내합니다. 끄면 텍스트 프롬프트만 사용합니다.',
```
`ui.es.ts`:
```ts
  settingsMotionLibrary: 'Usar referencia de movimiento (beta)',
  settingsMotionLibraryHint: 'Guía las animaciones con una plantilla de pose validada para acciones comunes. Desactívalo para usar solo prompts de texto.',
```

Match the exact key-quoting and trailing-comma style already in each file.

- [ ] **Step 3: Add the toggle row to `SettingsModal.tsx`**

Read `frontend/src/components/SettingsModal.tsx` to learn its state pattern (how it calls `GetSettings`, how existing controls call backend methods like `SetProvider`, and the translation function name, e.g. `t(...)`).

Add local state initialized from settings:
```tsx
const [motionLibrary, setMotionLibrary] = useState(true);
```
In the effect/handler that loads `GetSettings()` results, set it:
```tsx
setMotionLibrary(info.motionLibrary);
```
Add a toggle control near the provider/model controls (match the file's existing control markup; this is the shape):
```tsx
<label className="flex items-center justify-between gap-3">
  <span>
    <div>{t('settingsMotionLibrary')}</div>
    <div className="text-xs opacity-70">{t('settingsMotionLibraryHint')}</div>
  </span>
  <input
    type="checkbox"
    checked={motionLibrary}
    onChange={async (e) => {
      const v = e.target.checked;
      setMotionLibrary(v);
      await SetMotionLibrary(v);
    }}
  />
</label>
```
Ensure `SetMotionLibrary` is imported from the Wails bindings alongside the existing imports (e.g. `import { GetSettings, SetProvider, SetMotionLibrary } from '../../wailsjs/go/main/App';` — match the actual relative path used in the file).

- [ ] **Step 4: Typecheck and build the frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: no type errors.

- [ ] **Step 5: Manual verification**

Run the app (`./dev.sh` or `wails dev`). Open Settings → confirm the "Use motion reference (beta)" toggle appears, defaults ON, and persists across an app restart (toggle off, restart, still off). Generate `attack` with it ON, then OFF, and compare the result and the `motion` score shown in the states panel.

- [ ] **Step 6: Commit**

```bash
git add frontend/
git commit -m "$(printf 'Claude-5: Add motion-reference toggle to Settings UI [kanban-LmE2cl]\n\nCo-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>')"
```

---

## Final verification

- [ ] Run the full Go suite: `go test ./...` → all PASS.
- [ ] Build: `go build ./...` and `cd frontend && npx tsc --noEmit` → clean.
- [ ] Toggle OFF produces byte-identical generation behavior to pre-change (no extra ref, no clause) — confirmed by the `libraryTemplate` test and a manual OFF run.

---

## Self-Review

**Spec coverage:**
- Embedded library (~11 actions) → Task 2. ✓
- Config flag default-ON via `*bool` → Task 3. ✓
- `GenerateState` wiring + precedence (RefStrip wins) → Task 4 (`libraryTemplate` + clause branch). ✓
- Shared `PoseTemplateClause` → Task 1. ✓
- Frontend toggle + i18n (4 locales) + bindings → Task 5. ✓
- README provenance/IP caveat → Task 2 Step 7. ✓
- Testing (motionlib, config, sprite golden, app helper) → Tasks 1-4. ✓
- Out-of-scope items (per-action checkbox, upload, our-own-art, directional templates) → none added. ✓

**Type consistency:** `Template(string)([]byte,bool)`, `Covered()[]string`, `BaseStateName(string)string`, `PoseTemplateClause(string,int)string`, `MotionLibraryEnabled()bool`, `libraryTemplate(string,bool,bool)([]byte,bool)`, `SetMotionLibrary(bool)error`, `SettingsInfo.MotionLibrary bool` — names and signatures are used identically across tasks. ✓

**Placeholder scan:** every code step shows full code; no TBD/TODO; the one frontend ambiguity (exact existing markup/translation-fn) is handled by a "read the file to match its pattern" instruction plus a concrete shape, not a placeholder. ✓
