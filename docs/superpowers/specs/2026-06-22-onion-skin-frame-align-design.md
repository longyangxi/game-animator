# Onion-Skin Per-Frame Align — Design

**Date:** 2026-06-22
**Status:** Approved (design), pending implementation plan

## Problem

Automatic normalization in `ExtractFrames` can't satisfy every animation in a fixed cell.
With per-frame fit, a compact pose (e.g. a crouched pickup frame) is scaled up to fill its
cell and looks **too big** relative to its neighbors. A uniform-scale algorithm fixes the
pulse but then either **clips the weapon** (body-box scale) or **shrinks the whole character**
(full-content scale) — a genuine trilemma in a fixed cell.

Rather than chase a perfect algorithm, give the user **manual control**: a per-frame
transform (scale + position) adjusted visually with **onion-skin** ghosts of the neighboring
frames, the way Adobe Flash aligns animation frames. The automatic extraction stays as the
starting point; the user nudges the occasional bad frame.

## Scope

In scope:
- Per-frame **scale + position** transform (no rotation/flip).
- A dedicated **Align modal** with a large canvas, drag-to-move, scroll/slider-to-scale,
  arrow-key 1px nudges, reset.
- **Onion skin**: previous + next frame as faint ghosts behind the edited frame, on/off toggle.
- Transform applied **live** in the preview/player and **baked** at export.
- **Non-destructive**: the transform is stored, re-editable, resettable; the source frame PNG
  is never mutated.
- Persistence in the session JSON.

Out of scope (YAGNI): rotation, flip, per-frame color edits, tinted onion skin (prev=red/
next=green), configurable onion-skin depth/opacity, re-extraction from the raw strip.
These can be added later if missed.

## Architecture

**Frontend draw-time + export-bake.** The transform is a pure frontend concern.
- Stored per `FrameItem` in the session.
- Applied at **draw time** (nearest-neighbor) in the Align modal canvas, the frame thumbnails,
  and `AnimPlayer` — so the loop reflects edits immediately, with no baking on every drag.
- **Baked** only at export: the frontend draws each transformed frame onto a fresh
  cell-sized canvas (NN) → PNG dataURL → fed to the existing `ExportProject`. **No Go change.**

Rejected alternatives: backend-apply (more moving parts, no gain — the frontend already holds
the cell PNGs); re-extract-from-raw-strip per frame (highest fidelity, far more complex, overkill).

## Data model

```ts
// frontend/src/types.ts
export interface FrameTransform {
  scale: number; // relative to the frame content; 0.5–1.5; default 1
  dx: number;    // cell-pixel offset, default 0
  dy: number;    // cell-pixel offset, default 0
}

export interface FrameItem {
  id: string;
  png: string;        // dataURL — the source frame, never mutated
  selected: boolean;
  transform?: FrameTransform; // undefined == identity (1/0/0)
}
```

- **Scale anchor:** bottom-center (feet). Scaling about the feet keeps the character grounded;
  the user fine-tunes with `dx/dy`.
- `undefined` transform is treated as identity everywhere, so existing sessions load unchanged.

## Components

### 1. Transform helper (`frontend/src/lib/frameTransform.ts`)
Pure functions, unit-testable, no React:
- `identityTransform(): FrameTransform`
- `isIdentity(t?): boolean`
- `applyTransform(ctx, img, t, cellSize)` — draws `img` into a `cellSize` square at scale `t.scale`
  about the bottom-center, offset by `t.dx/t.dy`, with `imageSmoothingEnabled = false` (NN).
- `bakeTransformed(png, t, cellSize): Promise<string>` — renders to an offscreen canvas, returns a PNG dataURL.

### 2. Align modal (`frontend/src/components/AlignModal.tsx`)
- Opens for a `{state, frameIndex}`. Large square canvas (cell scaled up for visibility).
- Layers: cell border + baseline guide → onion ghosts (prev/next at low alpha) → current frame.
- Interaction: **drag** updates `dx/dy`; **scroll / slider** updates `scale` (0.5–1.5);
  **arrow keys** nudge `dx/dy` by 1px. **Onion toggle**, **Reset**, **Done/Cancel**.
- Local draft transform; commits to the store on **Done** (Cancel discards).

### 3. Preview & player (apply at draw time)
- `AnimPlayer` gains an optional `transforms?: (FrameTransform|undefined)[]` parallel to `frames`,
  and applies each via the helper when drawing (NN). Identity when absent → current behavior.
- Frame thumbnails in the preview panel render through the same helper so the grid matches.

### 4. Export bake
- In `handleExport` (`App.tsx`), before calling `ExportProject`, map each selected frame through
  `bakeTransformed(png, transform, cellSize)` and pass the baked dataURLs. Frames with identity
  transform pass through unbaked (cheap fast-path).

### 5. Persistence
- `transform` is part of `FrameItem`, already serialized by `SaveSession`/`LoadSession`.
  `LoadSession` tolerates missing `transform` (older sessions) → identity.

## Entry point / UX wiring
- A small **align icon button** on each frame thumbnail (appears on hover, like the existing
  exclude/checkbox controls) opens the Align modal for that frame.
- A frame with a non-identity transform shows a subtle badge so the user knows it was adjusted.

## Edge cases
- **Single-frame state / first / last frame:** onion skin shows only the available neighbor(s);
  none for a 1-frame state.
- **Excluded (deselected) frames:** onion skin uses the displayed/selected sequence (matches the
  exported order), so ghosts reflect what actually animates.
- **Scale up of cell-res pixels:** nearest-neighbor keeps pixels crisp (accepted blur-free tradeoff).
- **Reset:** sets transform to identity (or `undefined`), removing the badge.
- **Direction sets:** each direction's frames carry their own transforms (transforms live on the
  per-direction `FrameItem`s, so no special handling).

## Testing
- **Unit (helper):** `isIdentity`, and `bakeTransformed` geometry — scale-down centers/grounds
  correctly; `dx/dy` offsets land where expected; identity is a pixel-faithful pass-through.
  (Canvas in jsdom or a thin abstraction over the 2D context.)
- **Component (AlignModal):** drag updates `dx/dy`; scroll updates `scale`; arrow keys nudge 1px;
  Reset restores identity; Cancel discards, Done commits.
- **Integration (export):** a frame with a known transform bakes to the expected pixels; identity
  frames pass through unchanged.
- **Manual:** the pickup animation — shrink #2 with onion skin until it matches neighbors; confirm
  the player and exported GIF/APNG reflect it.

## Success criteria
- The user can open any frame, see prev/next ghosts, and scale/position it to match.
- Adjustments show live in the player and survive save/reload.
- Export (sheet, GIF, APNG, frames) reflects the adjustments.
- Pixels stay crisp (NN). Existing sessions load unchanged (identity default).
