# 2D / 2.5D Perspective Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a project-level 2D/2.5D perspective toggle so a project can be generated in classic isometric/top-down ¾ view, while 2D (the default) reproduces today's output byte-for-byte.

**Architecture:** A `perspective` value (`"flat"` default, `"iso"`) lives on the frontend character config, rides the existing request path (`GenerateCharacterArgs` / `GenerateStateArgs`) into the Go prompt builders, and selects perspective-specific prompt text. The flat path runs the unchanged code and is locked to current output by golden tests; all isometric wording lives behind an `iso` branch so it can be removed cleanly.

**Tech Stack:** Go (backend, `internal/sprite`), Wails (Go↔JS bridge, JSON args), React + TypeScript (frontend), i18n dictionary (en/es/ko/zh).

## Global Constraints

- **Losslessness:** when `perspective` is `""` or `"flat"`, `BuildCharacterPrompt`, `BuildStripPrompt`, and `FacingPromptSection` MUST return byte-identical output to the pre-change code. Enforced by golden tests.
- **Camera (iso):** classic 2.5D isometric / top-down three-quarter; fixed high camera tilted down **about 35 degrees**; orthographic pixel-isometric; no vanishing-point perspective, no photographic foreshortening, no 3D render.
- **8-direction system unchanged:** `Directions`, `GeneratedDirections`, and mirror pairs are untouched; iso only adds a downward tilt on top of each direction's yaw.
- **Default value string is `"flat"`** in the frontend; backend treats empty string and `"flat"` identically as flat.
- **Go package:** all backend code is `package sprite`; helpers defined in one file are visible across the package.
- **Run Go tests with:** `go test ./internal/sprite/...`

---

### Task 1: Golden safety net for current (flat) prompt output

Characterizes today's prompt output so later refactors can prove they changed nothing. Uses the **current** builder signatures (no `perspective` param yet).

**Files:**
- Create: `internal/sprite/prompt_golden_test.go`
- Create (generated): `internal/sprite/testdata/*.golden`

**Interfaces:**
- Consumes (current signatures): `BuildCharacterPrompt(description, style string) string`, `BuildStripPrompt(description, style string, spec StateSpec, feedback string) string`, `FacingPromptSection(key string) string`.
- Produces: `checkGolden(t *testing.T, name, got string)` helper and `-update-golden` flag, reused by Task 3.

- [ ] **Step 1: Write the golden test + helper**

Create `internal/sprite/prompt_golden_test.go`:

```go
package sprite

import (
	"flag"
	"os"
	"path/filepath"
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
	checkGolden(t, "character_flat", BuildCharacterPrompt("a knight", StylePresets["pixel"]))

	specEast := StateSpec{Name: "walk", Frames: 6, FPS: 10, Loop: true, Action: "walking", Facing: "east"}
	checkGolden(t, "strip_flat_east", BuildStripPrompt("a knight", StylePresets["pixel"], specEast, "make arms bigger"))

	specNoFacing := StateSpec{Name: "attack", Frames: 5, FPS: 12, Loop: false, Action: "melee attack", Facing: ""}
	checkGolden(t, "strip_flat_nofacing", BuildStripPrompt("a knight", StylePresets["pixel"], specNoFacing, ""))

	checkGolden(t, "facing_south", FacingPromptSection("south"))
	checkGolden(t, "facing_north", FacingPromptSection("north"))
}
```

- [ ] **Step 2: Generate the golden files**

Run: `go test ./internal/sprite -run TestPromptGoldenFlat -update-golden`
Expected: PASS, and `internal/sprite/testdata/` now contains `character_flat.golden`, `strip_flat_east.golden`, `strip_flat_nofacing.golden`, `facing_south.golden`, `facing_north.golden`.

- [ ] **Step 3: Verify the golden test passes against committed goldens**

Run: `go test ./internal/sprite -run TestPromptGoldenFlat -v`
Expected: PASS (no `-update-golden`, compares against the files just written).

- [ ] **Step 4: Commit**

```bash
git add internal/sprite/prompt_golden_test.go internal/sprite/testdata
git commit -m "test: golden snapshot of current flat prompt output [kanban-MK-1HO]"
```

---

### Task 2: Thread `perspective` through the builders (flat = byte-identical)

Adds the `perspective` parameter to all three builders. The flat/empty path must produce identical output; the golden test from Task 1 proves it. No isometric text yet.

**Files:**
- Modify: `internal/sprite/prompt.go` (`BuildCharacterPrompt`, `BuildStripPrompt`, add `isIso`, `perspectiveCharacterBullet`, `perspectiveStripClause`)
- Modify: `internal/sprite/direction.go` (`FacingPromptSection`)
- Modify: `app.go:304`, `app.go:416` (pass `""` for now)
- Modify: `internal/sprite/prompt_golden_test.go` (append `, ""` to builder calls)
- Modify: `internal/sprite/direction_test.go` (lines 30, 65, 68, 71, 79, 84)
- Modify: `internal/sprite/pipeline_test.go` (line 166)
- Modify: `internal/sprite/live_test.go` (line 47)

**Interfaces:**
- Produces (new signatures): `BuildCharacterPrompt(description, style, perspective string) string`, `BuildStripPrompt(description, style string, spec StateSpec, feedback, perspective string) string`, `FacingPromptSection(key, perspective string) string`.
- Produces: `isIso(perspective string) bool` (in `prompt.go`).
- Consumes: golden files + `checkGolden` from Task 1.

- [ ] **Step 1: Add perspective helpers in `prompt.go`**

Add near the top of `internal/sprite/prompt.go` (after the imports, e.g. above `StylePresets`):

```go
// isIso reports whether the perspective selects the 2.5D isometric view.
// Empty string and "flat" both mean the default 2D view.
func isIso(perspective string) bool {
	return strings.EqualFold(strings.TrimSpace(perspective), "iso")
}

// perspectiveCharacterBullet returns the framing bullet for the base-character prompt.
// The flat branch returns the exact text the prompt used before this parameter existed.
func perspectiveCharacterBullet(perspective string) string {
	if isIso(perspective) {
		return "- Classic 2.5D isometric / top-down three-quarter game-sprite view: a fixed high camera tilted down about 35 degrees so the tops of the head and shoulders read and the figure stands on an implied isometric ground plane. Keep it orthographic pixel-isometric — one flat consistent scale, no vanishing-point perspective, no photographic foreshortening, no 3D render.\n"
	}
	return "- Almost flat 2D game-sprite view; avoid dramatic perspective, foreshortening, cinematic camera angles, and illustration-style posing.\n"
}

// perspectiveStripClause returns an extra view-lock clause for the per-state strip prompt.
// It is empty for flat so the flat strip prompt is unchanged.
func perspectiveStripClause(perspective string) string {
	if isIso(perspective) {
		return "Isometric view lock: render every pose in a classic 2.5D isometric / top-down three-quarter view — a fixed high camera tilted down about 35 degrees, orthographic pixel-isometric, no vanishing-point perspective and no 3D render. Hold this exact downward tilt in every pose.\n\n"
	}
	return ""
}
```

- [ ] **Step 2: Use the helper in `BuildCharacterPrompt`**

In `internal/sprite/prompt.go`, change the signature and replace the hardcoded flat-view bullet.

Signature (line ~98):
```go
func BuildCharacterPrompt(description, style, perspective string) string {
```

Replace this line (currently `prompt.go:111`):
```go
	b.WriteString("- Almost flat 2D game-sprite view; avoid dramatic perspective, foreshortening, cinematic camera angles, and illustration-style posing.\n")
```
with:
```go
	b.WriteString(perspectiveCharacterBullet(perspective))
```

- [ ] **Step 3: Use the helper in `BuildStripPrompt`**

Signature (line ~120):
```go
func BuildStripPrompt(description, style string, spec StateSpec, feedback, perspective string) string {
```

Insert the strip clause between the style-contracts block and the facing section. Find:
```go
	if extra := pixelStyleContracts(style); extra != "" {
		b.WriteString(extra)
		b.WriteString("\n")
	}

	if sec := FacingPromptSection(spec.Facing); sec != "" {
```
Change it to:
```go
	if extra := pixelStyleContracts(style); extra != "" {
		b.WriteString(extra)
		b.WriteString("\n")
	}

	b.WriteString(perspectiveStripClause(perspective))

	if sec := FacingPromptSection(spec.Facing, perspective); sec != "" {
```
(`perspectiveStripClause` returns `""` for flat, so `b.WriteString("")` writes nothing — flat output is unchanged.)

- [ ] **Step 4: Add the perspective param to `FacingPromptSection`**

In `internal/sprite/direction.go`, change the signature and append the iso tilt line only for iso. Replace the whole function body (currently `direction.go:102-113`):

```go
func FacingPromptSection(key, perspective string) string {
	d, ok := facingDescs[key]
	if !ok {
		return ""
	}
	s := "Facing direction lock (overrides any other facing or view instruction in this prompt):\n" +
		"- Required view: " + d.view + " — " + d.camera + ".\n" +
		"- Body orientation: " + d.body + ".\n" +
		"- Visibility: " + d.visibility + ".\n" +
		"- The attached reference image shows this character from the front; redraw the IDENTICAL character (same hair, outfit, colors, proportions) rotated to this view.\n" +
		"- Every frame in the strip must use this exact same viewing angle. Never drift back toward a front view and never mirror the character between frames.\n"
	if isIso(perspective) {
		s += "- Isometric tilt: on top of the facing above, view from a fixed high three-quarter camera tilted down about 35 degrees — head and shoulder tops visible, figure on an implied isometric ground plane, the same downward tilt in every frame.\n"
	}
	return s
}
```

- [ ] **Step 5: Update non-test callers to pass `""`**

In `app.go`:
- Line ~304: `prompt := sprite.BuildCharacterPrompt(args.Description, style)` → `prompt := sprite.BuildCharacterPrompt(args.Description, style, "")`
- Line ~416: `prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback)` → `prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, "")`

(These become `args.Perspective` in Task 4.)

- [ ] **Step 6: Update test callers to the new signatures**

`internal/sprite/prompt_golden_test.go` — append `, ""` to each builder call in `TestPromptGoldenFlat`:
```go
	checkGolden(t, "character_flat", BuildCharacterPrompt("a knight", StylePresets["pixel"], ""))
	checkGolden(t, "strip_flat_east", BuildStripPrompt("a knight", StylePresets["pixel"], specEast, "make arms bigger", ""))
	checkGolden(t, "strip_flat_nofacing", BuildStripPrompt("a knight", StylePresets["pixel"], specNoFacing, "", ""))
	checkGolden(t, "facing_south", FacingPromptSection("south", ""))
	checkGolden(t, "facing_north", FacingPromptSection("north", ""))
```

`internal/sprite/direction_test.go`:
- Line 30: `FacingPromptSection(d.Key)` → `FacingPromptSection(d.Key, "")`
- Line 65: `FacingPromptSection("")` → `FacingPromptSection("", "")`
- Line 68: `FacingPromptSection("west")` → `FacingPromptSection("west", "")`
- Line 71: `FacingPromptSection("north")` → `FacingPromptSection("north", "")`
- Line 79: `BuildStripPrompt("a knight", StylePresets["pixel"], spec, "")` → `BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "")`
- Line 84: `BuildStripPrompt("a knight", StylePresets["pixel"], spec, "")` → `BuildStripPrompt("a knight", StylePresets["pixel"], spec, "", "")`

`internal/sprite/pipeline_test.go`:
- Line ~166-168: the call ends with `}, "make arms bigger")` → change to `}, "make arms bigger", "")`

`internal/sprite/live_test.go`:
- Line 47: `BuildStripPrompt(desc, style, spec, feedback)` → `BuildStripPrompt(desc, style, spec, feedback, "")`

- [ ] **Step 7: Verify losslessness — golden test still passes WITHOUT updating goldens**

Run: `go test ./internal/sprite -run TestPromptGoldenFlat -v`
Expected: PASS. (If it fails, the flat path changed — fix the code, do NOT run `-update-golden`.)

- [ ] **Step 8: Verify the whole package builds and tests pass**

Run: `go build ./... && go test ./internal/sprite/...`
Expected: build succeeds; tests PASS (`TestFacingPromptSection`, `TestBuildStripPromptIncludesFacing`, `TestBuildStripPrompt`, `TestDirections...` all green with the new signatures).

- [ ] **Step 9: Commit**

```bash
git add internal/sprite/prompt.go internal/sprite/direction.go app.go internal/sprite/prompt_golden_test.go internal/sprite/direction_test.go internal/sprite/pipeline_test.go internal/sprite/live_test.go
git commit -m "refactor: thread perspective param through prompt builders (flat byte-identical) [kanban-MK-1HO]"
```

---

### Task 3: Isometric smoke test

Locks in that iso output differs from flat and carries the isometric markers. No quality assertion — 2.5D quality is judged visually.

**Files:**
- Modify: `internal/sprite/prompt_golden_test.go` (add `TestPromptIso`)

**Interfaces:**
- Consumes: the new builder signatures from Task 2 and `isIso`.

- [ ] **Step 1: Write the iso smoke test**

Append to `internal/sprite/prompt_golden_test.go`:

```go
import "strings" // add to the existing import block if not already present

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
```

- [ ] **Step 2: Run the iso smoke test**

Run: `go test ./internal/sprite -run TestPromptIso -v`
Expected: PASS.

- [ ] **Step 3: Confirm flat goldens are still untouched**

Run: `go test ./internal/sprite -run TestPromptGoldenFlat -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/sprite/prompt_golden_test.go
git commit -m "test: isometric prompt smoke test [kanban-MK-1HO]"
```

---

### Task 4: Backend request args carry perspective

Adds the `Perspective` field to both generation arg structs and feeds it into the builders.

**Files:**
- Modify: `app.go` (`GenerateCharacterArgs` ~292, `GenerateStateArgs` ~336, builder calls ~304 and ~416)

**Interfaces:**
- Produces: JSON field `perspective` on both args; backend passes it to `BuildCharacterPrompt` / `BuildStripPrompt`.
- Consumes: builder signatures from Task 2.

- [ ] **Step 1: Add the field to `GenerateCharacterArgs`**

In `app.go`, in the `GenerateCharacterArgs` struct (~292-296), add after `StyleCustom`:
```go
	Perspective string `json:"perspective"` // "" / "flat" = 2D, "iso" = 2.5D
```

- [ ] **Step 2: Add the field to `GenerateStateArgs`**

In `app.go`, in the `GenerateStateArgs` struct (~336-346), add after `StyleCustom`:
```go
	Perspective string `json:"perspective"` // "" / "flat" = 2D, "iso" = 2.5D
```

- [ ] **Step 3: Pass the field into the builders**

In `app.go`:
- `prompt := sprite.BuildCharacterPrompt(args.Description, style, "")` → `prompt := sprite.BuildCharacterPrompt(args.Description, style, args.Perspective)`
- `prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, "")` → `prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, args.Perspective)`

- [ ] **Step 4: Verify build + tests**

Run: `go build ./... && go test ./internal/sprite/...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add app.go
git commit -m "feat: accept perspective in generation requests [kanban-MK-1HO]"
```

---

### Task 5: Frontend type + persistence

Adds `perspective` to the character config type and the three places the character object is constructed, with a load-time fallback. No UI yet.

**Files:**
- Modify: `frontend/src/types.ts` (`CharacterDef` ~61-67)
- Modify: `frontend/src/App.tsx` (default ~46, load fallback ~100, reset ~492)

**Interfaces:**
- Produces: `CharacterDef.perspective: "flat" | "iso"`, always defined (default `"flat"`).

- [ ] **Step 1: Ensure frontend deps are installed**

Run: `cd frontend && npm install`
Expected: completes; `frontend/node_modules` exists. (Required for the type-check in later steps.)

- [ ] **Step 2: Add the field to `CharacterDef`**

In `frontend/src/types.ts`, inside `interface CharacterDef`, add after `styleCustom: string;`:
```ts
  perspective: "flat" | "iso"; // 2D (default) or 2.5D isometric
```

- [ ] **Step 3: Set the default in the three construction sites**

In `frontend/src/App.tsx`:

Initial state (~46, the object with `styleKey: "pixel", styleCustom: "",`) — add:
```ts
    perspective: "flat",
```

Load fallback (~100, the object with `styleKey: s.character.styleKey ?? "pixel",`) — add:
```ts
              perspective: s.character.perspective ?? "flat",
```

Reset (~492): change
```ts
    setCharacter({ image: null, name: "", description: "", styleKey: "pixel", styleCustom: "" });
```
to
```ts
    setCharacter({ image: null, name: "", description: "", styleKey: "pixel", styleCustom: "", perspective: "flat" });
```

- [ ] **Step 4: Type-check**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors in `types.ts` or `App.tsx`. (A `CharacterDef` missing `perspective` would now be a type error — confirms all construction sites are covered.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types.ts frontend/src/App.tsx
git commit -m "feat: add perspective to character config + persistence [kanban-MK-1HO]"
```

---

### Task 6: Frontend UI toggle + wire perspective into requests + i18n

Adds the View selector under the Style block and sends `perspective` in both generation calls.

**Files:**
- Modify: `frontend/src/components/CharacterPanel.tsx` (Style block ~151-167; `generateBase` call ~55-59)
- Modify: `frontend/src/App.tsx` (`GenerateState` args ~225-233)
- Modify: `frontend/src/i18n/ui.en.ts`, `ui.zh.ts`, `ui.es.ts`, `ui.ko.ts`

**Interfaces:**
- Consumes: `CharacterDef.perspective` (Task 5), backend `perspective` JSON field (Task 4).

- [ ] **Step 1: Add i18n keys (4 locales)**

Add next to `refine_title` (line ~126) in each file. The English keys also serve as fallback for any locale.

`frontend/src/i18n/ui.en.ts`:
```ts
  view_label: "View",
  view_flat: "2D (flat)",
  view_iso: "2.5D (isometric)",
  view_hint: "Regenerate the character and animations to apply a view change.",
```
`frontend/src/i18n/ui.zh.ts`:
```ts
  view_label: "视角",
  view_flat: "2D（平面）",
  view_iso: "2.5D（等距）",
  view_hint: "切换视角后需重新生成角色和动画才生效。",
```
`frontend/src/i18n/ui.es.ts`:
```ts
  view_label: "Vista",
  view_flat: "2D (plano)",
  view_iso: "2.5D (isométrico)",
  view_hint: "Regenera el personaje y las animaciones para aplicar el cambio de vista.",
```
`frontend/src/i18n/ui.ko.ts`:
```ts
  view_label: "시점",
  view_flat: "2D (평면)",
  view_iso: "2.5D (등각)",
  view_hint: "시점 변경을 적용하려면 캐릭터와 애니메이션을 다시 생성하세요.",
```

- [ ] **Step 2: Add the View selector in `CharacterPanel.tsx`**

In `frontend/src/components/CharacterPanel.tsx`, immediately after the closing `</div>` of the art-style `field` block (after the style `<Select>`'s wrapping `<div className="field">…</div>`, i.e. after line ~167 and before the `{character.styleKey === "custom" && (` block at ~169), insert:

```tsx
      <div className="field">
        <Label>{t("view_label")}</Label>
        <Select value={character.perspective} onValueChange={(v) => set({ perspective: v as "flat" | "iso" })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="flat">{t("view_flat")}</SelectItem>
            <SelectItem value="iso">{t("view_iso")}</SelectItem>
          </SelectContent>
        </Select>
        {character.perspective === "iso" && <p className="hint">{t("view_hint")}</p>}
      </div>
```

(`Select`, `SelectTrigger`, `SelectValue`, `SelectContent`, `SelectItem`, `Label` are already imported in this file — confirm at the top; if `Label` or `Select*` is missing from the import, add it from the same module the style `Select` uses.)

- [ ] **Step 3: Send perspective in the base-character generation call**

In `frontend/src/components/CharacterPanel.tsx`, in `generateBase` (~55-59), add to the `GenerateCharacter({...})` object after `styleCustom: character.styleCustom,`:
```ts
        perspective: character.perspective,
```

- [ ] **Step 4: Send perspective in the per-state generation call**

In `frontend/src/App.tsx`, in the `GenerateState({...})` call (~225-233), add after `styleCustom: ch.styleCustom,`:
```ts
        perspective: ch.perspective,
```

- [ ] **Step 5: Type-check + build**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: no type errors; build succeeds.

- [ ] **Step 6: Manual smoke (real app)**

Run the app (e.g. `wails dev` from the repo root, or the project's usual run command). Verify:
1. The Character panel shows a **View** selector defaulting to **2D (flat)**.
2. Switching to **2.5D (isometric)** shows the regenerate hint.
3. Generating a character then an animation in **2D** looks the same as before (no regression).
4. Generating in **2.5D** produces a visibly tilted/isometric result.
(2.5D quality is evaluated by eye — if it underperforms, the iso branch can be removed per the spec's rollback plan without touching the flat path.)

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/CharacterPanel.tsx frontend/src/App.tsx frontend/src/i18n/ui.en.ts frontend/src/i18n/ui.zh.ts frontend/src/i18n/ui.es.ts frontend/src/i18n/ui.ko.ts
git commit -m "feat: 2D/2.5D view toggle in character panel [kanban-MK-1HO]"
```

---

## Self-Review

**Spec coverage:**
- Project-level setting on character config, default flat → Tasks 5, 6.
- Flows through existing request path → Task 4.
- Losslessness via golden tests → Tasks 1, 2 (Step 7), 3 (Step 3).
- Iso = isometric/top-down ¾, ~35° → Task 2 strings, Global Constraints.
- 8-direction semantics + mirroring unchanged → Task 2 only appends to facing sections; `Directions`/`MirrorNRGBA` untouched.
- base character also iso → Task 2 Step 2 + Task 4 Step 3 (BuildCharacterPrompt gets perspective).
- UI under Style → Task 6 Step 2.
- i18n 4 locales → Task 6 Step 1.
- Rollback isolated to iso branch → iso text confined to `perspectiveCharacterBullet`/`perspectiveStripClause`/`FacingPromptSection` iso block + UI option.

**Placeholder scan:** No TBD/TODO; all code shown in full.

**Type consistency:** Builder signatures `BuildCharacterPrompt(description, style, perspective string)`, `BuildStripPrompt(description, style string, spec StateSpec, feedback, perspective string)`, `FacingPromptSection(key, perspective string)` are used identically in Tasks 2, 3, 4. `isIso` defined once (Task 2 Step 1). Frontend `perspective: "flat" | "iso"` consistent across Tasks 5, 6. JSON key `perspective` matches between Go tags (Task 4) and JS payloads (Task 6).
