# Frame Edit Headroom (Padded Working Canvas) — Design

**Date:** 2026-06-23
**Status:** Approved (design), pending implementation plan
**Follows:** [2026-06-22-onion-skin-frame-align-design.md](./2026-06-22-onion-skin-frame-align-design.md)

## Problem

When a frame's content reaches the cell edge (e.g. a sword extended straight to one
side), there is almost no room to nudge that frame in the Align modal: moving the
character toward the sword pushes the sword past the cell boundary, where it is
**clipped** — both visually in the editor and permanently at export. The user needs
headroom around the cell to align such frames against their onion-skin neighbors
without losing content.

Note: `FrameTransform.dx/dy` are already unclamped, so the character can *already* be
moved anywhere in data terms. The limitation is purely two clip points:
1. `AlignModal.draw()` clears and bounds drawing to the `cellSize` canvas — overflow is invisible.
2. `bakeTransformed()` renders into a `cellSize × cellSize` canvas — overflow is discarded at export.

## Decision

Add a fixed **padding margin** around the cell so editing and export both operate on a
larger `cellSize + 2·pad` working canvas. **No shrink-back / fit-to-content pass** — it
was considered and rejected (see below).

### Why no fit-to-content shrink-back

The user's worry: a crop step done wrong scrambles the motion baseline and makes the
character jitter. That worry is correct *only* for a per-frame tight crop. A single
uniform crop rectangle applied to every frame cannot jitter (every frame translates by
the same amount). But a correct uniform crop's only benefit is removing harmless,
uniform transparent margin — low value, so by YAGNI we skip it. The padded size *is* the
final exported frame size.

## Core invariant (jitter-safety by construction)

Every frame, in every consumer, is placed by the **same** `computeDrawRect(…cellSize)`
used today, then translated by a **constant** `(pad, pad)` into the `cellSize + 2·pad`
canvas. The bottom-center anchor never moves and the offset is identical for all frames,
so there is **zero frame-to-frame relative motion** → no baseline drift, no jitter.
Padding only enlarges the "cell"; it never moves content relative to other content.

## Scope

In scope (all frontend; Go backend untouched):

1. **`frameTransform.ts`** — add:
   - `PAD_FRAC` constant (fraction of `cellSize` added on *each* side; default ~0.15) and
     `framePad(cellSize): number` helper (`Math.round(cellSize * PAD_FRAC)`).
   - An optional `pad = 0` argument to `computeDrawRect`, `applyTransform`, and
     `bakeTransformed`. With `pad = 0` behavior is byte-identical to today.
   - `computeDrawRect` adds `pad` to both `x` and `y` of its result (content keeps its
     existing position within the inner cell, shifted into the padded canvas).
   - `bakeTransformed` renders into a `(cellSize + 2·pad)` square. The identity fast-path
     (return raw PNG) is dropped when `pad > 0`, since padded frames must be re-composited.

2. **`AlignModal.tsx`** (the fix) — map the on-screen canvas to the padded size, draw the
   **original cell as an inset guide rectangle** (inset by `pad·k`), composite onion
   neighbors + current frame with the `pad` offset, and update the view↔content scale
   factor `k` to use the padded size. Dragged-out content stays visible and editable.

3. **`AnimPlayer.tsx` + `PreviewPanel` AtlasView** (preview parity) — size their canvases
   to the padded cell and pass `pad`. Required so the in-app player/atlas show the same
   content the export keeps (WYSIWYG); otherwise rescued content looks clipped in-app.

4. **`App.tsx` export** — bake with `pad` and pass `cellSize: cellSize + 2·pad` to
   `ExportProject`. The Go `ComposeAtlas` lays the atlas out by the cell size it is handed,
   so a larger value Just Works with no backend change.

Out of scope (YAGNI): fit-to-content / shrink-back crop, configurable per-project padding
UI, padding the filmstrip CSS thumbnails (cosmetic, left as-is), asymmetric padding,
rotation/flip.

## Untouched

Go backend, slicing/extraction (`ExtractFrames` still yields `cellSize` frames), the
immutable source PNGs, stored transforms, and the session format — `cellSize` saved stays
the original; `pad` is derived at runtime via `framePad(cellSize)`, so no migration.

## Testing

- Unit (`frameTransform.test.ts`, vitest): `framePad` rounding; `computeDrawRect` with
  `pad > 0` offsets x/y by exactly `pad` versus the `pad = 0` result and is otherwise
  unchanged; `pad = 0` is identical to current behavior (regression guard).
- Manual: a frame with content at the cell edge can be dragged so the overflow remains
  visible in the Align modal, the AnimPlayer/atlas show the same framing, and the exported
  sheet retains the overflow with no jitter across the loop.
