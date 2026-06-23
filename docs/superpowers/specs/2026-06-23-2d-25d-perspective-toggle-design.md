# 2D / 2.5D Perspective Toggle — Design

**Date:** 2026-06-23
**Status:** Approved (design), pending implementation plan

## Problem

Every animation this tool generates is a flat, eye-level 2D sprite. That is not a
model limitation — it is baked into the prompts:

- `BuildCharacterPrompt` (`internal/sprite/prompt.go:111`) hardcodes
  *"Almost flat 2D game-sprite view; avoid dramatic perspective, foreshortening,
  cinematic camera angles, and illustration-style posing."*
- Every entry in `facingDescs` (`internal/sprite/direction.go:67`) describes a camera
  that rotates only around the vertical axis (yaw) and is explicitly **at eye level** —
  there is no downward pitch. The "¾" directions (south-east, etc.) are a *horizontal*
  3/4 turn, not a top-down isometric 3/4.

The user wants the ability to switch the whole project to a **2.5D / classic
isometric (top-down ¾)** look — camera tilted down ~35°, ground plane and the top of
the character visible, like Diablo / Stardew Valley / RTS sprites.

## Decision

Add a **project-level perspective setting** with two values:

- `"flat"` (default) — **2D, byte-for-byte identical to today's behavior.**
- `"iso"` — 2.5D classic isometric / top-down ¾.

It lives on the character config alongside `styleKey`, flows through the same request
path as style, and feeds the prompt builders. Switching it requires regenerating the
character and animations to take effect (same as changing style).

**Hard requirement — losslessness:** when perspective is `"flat"` (or empty), the
prompt builders must return the *exact same strings* they return today. This is locked
down with golden tests, not just code review. Selecting 2D is guaranteed to reproduce
current output.

**Experimental / removable:** all 2.5D wording lives behind the `iso` branch. If it
doesn't pan out, removal touches only the iso branches, the iso descriptions, the UI
option, and the field default — the flat path is never modified.

## Camera definition (the "iso" profile)

Classic isometric / top-down ¾:
- Camera pitched **down ~35°** (a fixed high three-quarter angle), looking at the
  character from above-and-in-front for the front direction.
- Pixel-art **orthographic isometric** feel — no true 3D render, no vanishing-point
  perspective, no photographic foreshortening; consistent cell scale preserved.
- The top surfaces of head/shoulders are visible; the implied ground plane recedes
  upward. Shadows/contact patches stay dropped (the keying contract is unchanged).

### 8-direction semantics are unchanged

The 8-direction yaw system stays exactly as-is (`Directions`, `GeneratedDirections`,
mirror pairs in `direction.go:20`). Iso mode only **adds a downward pitch** on top of
each direction's existing yaw. Left/right mirroring (W = flipped E, etc. via
`MirrorNRGBA`) remains valid — isometric games mirror left/right by construction.

## Components & changes

### 1. Data model
- Frontend character config gains `perspective: "flat" | "iso"` (default `"flat"`),
  persisted with a load-time fallback `?? "flat"` (mirroring how `styleKey` is handled
  in `App.tsx`).
- `GenerateCharacterArgs` and `GenerateStateArgs` (`app.go:292`, `app.go:336`) each gain
  `Perspective string `json:"perspective"`` (empty string treated as flat).
- Go `StateSpec` is **not** modified — perspective is a project-level parameter passed
  alongside the spec, not a per-state field.

### 2. UI
- Add a `Select` in `CharacterPanel.tsx`, directly under the existing Style block
  (`CharacterPanel.tsx:151`): label "View / 视角", options `2D (flat, default)` and
  `2.5D (isometric)`.
- New i18n keys (`view_label`, `view_flat`, `view_iso`, and a short "regenerate to
  apply" hint) added to all four locales (en/es/ko/zh); missing keys already fall back
  to English.

### 3. Prompt builders (`internal/sprite/prompt.go`, `direction.go`)
Thread a `perspective string` argument into:
- `BuildCharacterPrompt(description, style, perspective)`
- `BuildStripPrompt(description, style, spec, feedback, perspective)`
- `FacingPromptSection(key, perspective)`

Branch structure (per builder):
- **flat / empty →** the existing code path runs unchanged and emits today's exact
  strings. No edits to the current string literals.
- **iso →** swap the flat-view clause in `BuildCharacterPrompt` for the iso clause, and
  in `FacingPromptSection` append the downward-pitch camera + top-surface visibility to
  each direction's description.

The iso descriptions are kept in one place (e.g. an `isoFacing` map / helper and an
`isoViewClause()` function) so the feature is self-contained and easy to delete.

### 4. base character also uses the perspective (decided)
When `perspective == "iso"`, `BuildCharacterPrompt` produces an isometric front-facing
reference so the identity anchor matches the animation strips. (Animation strips
reference the base image; a flat base + iso strips would fight each other and worsen
frame-to-frame drift.) Consequence: switching perspective mid-project requires
regenerating the base character and all animations — accepted, consistent with style.

## Losslessness guarantee (testing)

Add Go unit tests in `internal/sprite`:
1. **Golden snapshot** of current `BuildCharacterPrompt`, `BuildStripPrompt`, and
   `FacingPromptSection` output (captured *before* the signature change, using the
   pre-change call form).
2. Assert that with `perspective == ""` and `perspective == "flat"`, the new builders
   reproduce the golden output **exactly** (string equality).
3. A smoke test that `perspective == "iso"` output (a) differs from flat and
   (b) contains the iso markers (pitch / isometric wording) — no quality assertion,
   since 2.5D quality is evaluated manually.

Existing callers/tests that call the builders with the old signature are updated to pass
`""` (flat), which by the golden test is behavior-preserving.

## Out of scope / YAGNI
- No per-animation perspective (global only).
- No new style presets dedicated to iso (style and perspective are orthogonal).
- No automated quality scoring for iso output — judged visually by the user.
- No migration of already-generated flat projects — switching requires regeneration.

## Rollback plan
Remove: the `iso` branches in the three builders, the `isoFacing`/`isoViewClause`
helpers, the UI `Select` option (or the whole field), the args field, and the new i18n
keys. The flat path and golden tests are untouched, so 2D is provably unaffected.
