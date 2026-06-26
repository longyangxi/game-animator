# Motion-Reference Toggle — Design

**Date:** 2026-06-26
**Status:** Approved for planning
**Author:** mark + Claude

## Problem

Today every animation is generated purely from text prompts. A throwaway experiment
(`cmd/ppposetest`) showed that feeding a **validated pose sprite-sheet** as a second
reference image — alongside our character — produces a clearer, more coherent attack
motion while fully preserving our character's identity. Measured `motion` score and
visual read both improved:

| Variant | motion score |
|---|---|
| pose template + our text Hint | 0.362 |
| pose template only | 0.321 |
| control (text only, current behavior) | 0.293 |

The lift is real but modest, so we want to validate it in the real product before
committing. The mechanism must be a **switch the user can turn off** if the
improvement doesn't justify it.

## Goal

Add a single global toggle ("Use motion reference") that, when ON, injects a validated
pose-template image + a pose-transfer instruction into generation for a curated set of
common actions. When OFF, behavior is byte-identical to today. The toggle is trivially
removable if validation fails.

## Non-goals (v1)

- Per-action checkbox or a side-by-side compare mode (the global toggle + the existing
  `motion` score in `StateResult.Scores` is enough to compare by flip-and-regenerate).
- User-uploaded pose references.
- Building our own pose-template art (validation phase reuses borrowed sheets).
- Pose templates for directional / 8-direction sets (those keep their current
  same-character `RefStrip` behavior untouched).
- Covering all 100 presets — v1 covers ~10 common actions; uncovered actions fall back
  to text-only automatically.

## Decisions

- **Toggle placement:** global setting in the Settings screen (Approach 1). Matches the
  "one switch I can turn off" framing; smallest, most reversible change.
- **Default:** **ON**. Users see the lift by default and turn it off if unconvinced.
- **Library source:** front rows cropped from `image-cockpit-for-codex-workflows`
  official animation sheets. **Validation-phase placeholders only** — flagged in a
  library README to be replaced with our own art before any public release (their
  license is marked `sample`; the templates are an internal generation *input*, never
  shipped in user output).
- **Coverage (~11 actions):** their action → our preset name —
  `idle, walk, run, attack, hurt, death, cast, jump, cheer, knockback` map 1:1;
  `guard → block`.

## Architecture

Four units, each independently testable.

### 1. `internal/motionlib` — embedded pose-template library

- Holds front-row pose strips as PNGs, embedded with `//go:embed`. We commit only the
  cropped **front rows** (the `cropRow(sheet, 5, 0)` logic from the experiment), not the
  full 5-direction sheets, so the binary carries only what it uses.
- Public API:
  - `Template(stateName string) ([]byte, bool)` — lowercases, strips any direction
    suffix (reuse `sprite.stripDirectionSuffix` semantics; expose a helper if needed),
    maps to a covered action, returns PNG bytes or `(nil, false)`.
  - `Covered() []string` — list of covered preset names (for tests / future UI).
- `internal/motionlib/README.md` documents provenance and the "replace before release"
  caveat, plus the source→preset name mapping.

### 2. Config flag — `internal/config`

- Add `MotionLibrary *bool json:"motionLibrary,omitempty"` to `Settings`. A pointer so
  "unset" (new users / old config files) defaults to ON, while an explicit `false`
  persists an off choice.
- Add `func (s *Settings) MotionLibraryEnabled() bool { return s.MotionLibrary == nil || *s.MotionLibrary }`.

### 3. Backend wiring — `app.go`

- Promote the experiment's clause into the sprite package as
  `sprite.PoseTemplateClause(stateName string, frames int) string` (single source for
  app.go, the CLI, and tests). `cmd/ppposetest` switches to call it.
- In `GenerateState` (around lines 398–421), after the existing
  `refs := [][]byte{baseRaw}` and `RefStrip` append:
  ```
  if settings.MotionLibraryEnabled() && strings.TrimSpace(args.RefStrip) == "" {
      if tpl, ok := motionlib.Template(args.State.Name); ok {
          refs = append(refs, tpl)
          motionTemplate = true
      }
  }
  ```
  and inside the attempt loop, when `motionTemplate` is true, append
  `sprite.PoseTemplateClause(args.State.Name, args.State.Frames)` to the prompt.
- **Precedence (explicit):** the existing directional `RefStrip` (same-character motion
  ref for 8-direction sets) always wins — the library template is only used when
  `RefStrip` is empty. The existing `len(refs) > 1` directional clause and the new pose
  clause are therefore mutually exclusive per generation.
- `GenerateState` calls `config.Load()` once at the top to read the flag.

### 4. Frontend — `SettingsModal.tsx`

- Add a toggle row "Use motion reference (beta)" bound to:
  - read: `GetSettings().motionLibrary` (new `bool` field on `SettingsInfo`, computed
    via `MotionLibraryEnabled()` in `GetSettings`).
  - write: new Wails method `func (a *App) SetMotionLibrary(enabled bool) error`
    mirroring `SetProvider` (load → set `&enabled` → `config.Save`).
- i18n: add the label + helper text to all four locale files
  (`ui.en.ts`, `ui.es.ts`, `ui.ko.ts`, `ui.zh.ts`).
- Regenerate Wails bindings (`wailsjs/`) for the new `SetMotionLibrary` method.

## Data flow

```
SettingsModal toggle ──SetMotionLibrary(bool)──▶ config.Save(Settings.MotionLibrary)
                                                          │
GenerateState(args) ──config.Load()──▶ MotionLibraryEnabled()? ──┐
                                                                 │ yes & RefStrip empty
                          motionlib.Template(state) ─(bytes,ok)──┤
                                                                 ▼
   refs += template ; prompt += sprite.PoseTemplateClause(state, frames)
                                                                 ▼
                          provider.GenerateImage(prompt, refs, aspect)
```

## Error handling / edge cases

- Uncovered action or `Template` miss → no append, text-only path (no error).
- Corrupt/missing embedded asset → `Template` returns `(nil, false)`; generation
  proceeds text-only. Embedding is validated at build time, so this is defensive only.
- Directional set (`RefStrip` present) → library skipped by precedence rule.
- Flag off → zero behavior change; the `refs`/prompt are identical to today.

## Testing

- `internal/motionlib`: `Template` resolves each covered name; strips `-south` /
  compound direction suffixes; unknown name → `(nil,false)`; every embedded asset
  decodes as a valid PNG; `Covered()` matches the embedded files.
- `internal/config`: `MotionLibraryEnabled()` — nil → true, `&false` → false,
  `&true` → true; round-trips through `Save`/`Load`.
- `sprite`: golden test pinning `PoseTemplateClause` output.
- `app.go`: `GenerateState` appends the template ref + clause when flag on, template
  exists, and `RefStrip` empty; skips it when flag off, when `RefStrip` set, and for
  uncovered actions. (Use the existing provider stub pattern from `provider_test`.)
- Manual: flip toggle in Settings, regenerate `attack`, compare visual + `motion`
  score against the off run.

## Rollback

Set the toggle off (no behavior change), or delete `internal/motionlib`, the config
field, the `SetMotionLibrary` method, and the SettingsModal row. No data migration —
the config field is `omitempty`.

## Open follow-ups (post-validation)

- If validated: replace borrowed templates with our own QA'd art; expand coverage;
  consider directional templates and a user-upload override.
- If not: remove per "Rollback".
