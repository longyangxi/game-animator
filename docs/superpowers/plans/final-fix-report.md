# Final Fix Report — Onion-Skin Per-Frame Align

## Fix 1 (CRITICAL) — AlignModal renders dx/dy in cell-space instead of view-space

### Files touched
- `frontend/src/lib/frameTransform.ts` — added `scaleTransform` helper after `clampScale`
- `frontend/src/lib/frameTransform.test.ts` — added 2 new tests for `scaleTransform` (TDD: written before implementation, confirmed failing, then passing)
- `frontend/src/components/AlignModal.tsx` — updated `draw()` to use `scaleTransform` for both active frame and onion neighbors; added `items` to `useCallback` dependency array

### What changed
`scaleTransform(t, k)` converts a `FrameTransform` whose `dx/dy` are in cell pixels into one appropriate for a canvas scaled by `k = VIEW / cellSize`. Previously `AlignModal.draw()` was passing `draftRef.current` (cell-space) raw to `applyTransform` with `VIEW` as `cellSize`, making offsets render ~0.71x too small and drag lag the cursor. Onion neighbor ghosts were drawn with `ctx.drawImage` at identity (ignoring their stored transforms).

After the fix:
- Active frame: `applyTransform(ctx, im, im.width * k, im.height * k, scaleTransform(draftRef.current, k), VIEW)`
- Each neighbor: `applyTransform(ctx, im, im.width * k, im.height * k, scaleTransform(items[i].transform, k), VIEW)` (inside `globalAlpha = 0.25` block)

---

## Fix 2 (IMPORTANT) — Atlas preview ignores transforms

### Files touched
- `frontend/src/components/PreviewPanel.tsx`

### What changed
`AtlasView`'s async image-loading `Promise` type changed from `Promise<HTMLImageElement>` to `Promise<{ img: HTMLImageElement; transform: FrameTransform | undefined }>`, carrying each frame's transform alongside its image. The draw loop now uses `ctx.save()` / `ctx.translate(ci * cellSize, ri * cellSize)` / `applyTransform(...)` / `ctx.restore()` so the preview matches the exported sheet.

Import updated: added `applyTransform` to the existing `isIdentity` import from `../lib/frameTransform`.

---

## Fix 3 (MINOR) — Arrow keys steal the scale slider's increment

### Files touched
- `frontend/src/components/AlignModal.tsx`

### What changed
Added an early-return guard at the top of the `keydown` handler:
```ts
if ((e.target as HTMLElement)?.tagName === "INPUT") return;
```
When the `<input type="range">` is focused, arrow keys now operate the slider normally instead of being intercepted by `preventDefault()`.

---

## Fix 4 (MINOR) — Document bakeTransformed invariant

### Files touched
- `frontend/src/lib/frameTransform.ts`

### What changed
Added one-line comment above the `loadImage` call in `bakeTransformed`:
```ts
// assumes the source PNG is cell-sized (frames are generated at cell resolution)
```

---

## Test command and output

```
cd frontend && npm test
```

```
 RUN  v0.34.6 /Users/marklong/.open-office/worktrees/perfectpixel-studio-5qq9o8/kanban-IAp-kA/frontend

 ✓ src/lib/frameTransform.test.ts > identityTransform / isIdentity > identity is scale 1, no offset
 ✓ src/lib/frameTransform.test.ts > identityTransform / isIdentity > undefined and identity values are identity
 ✓ src/lib/frameTransform.test.ts > identityTransform / isIdentity > any non-identity value is not identity
 ✓ src/lib/frameTransform.test.ts > clampScale > clamps to [SCALE_MIN, SCALE_MAX]
 ✓ src/lib/frameTransform.test.ts > computeDrawRect (bottom-center anchor) > identity fills the cell
 ✓ src/lib/frameTransform.test.ts > computeDrawRect (bottom-center anchor) > scale shrinks about the bottom-center
 ✓ src/lib/frameTransform.test.ts > computeDrawRect (bottom-center anchor) > offset shifts the rect
 ✓ src/lib/frameTransform.test.ts > scaleTransform > scales dx/dy by k, leaves scale
 ✓ src/lib/frameTransform.test.ts > scaleTransform > passes undefined through
 ✓ src/lib/frameTransform.test.ts > applyTransform > disables smoothing and draws at the computed rect

 Test Files  1 passed (1)
      Tests  10 passed (10)
   Start at  14:38:14
   Duration  173ms
```

**2 new `scaleTransform` tests added; all 10 pass.**

---

## tsc result

```
node_modules/.bin/tsc --noEmit
```
Exit code: 0 (no output, no errors).

---

## Build result

```
npm run build
```

```
> tsc && vite build
vite v3.2.11 building for production...
✓ 1858 modules transformed.
dist/assets/index.51a12cad.js    373.06 KiB / gzip: 118.50 KiB
```

Build succeeded. The `'use client'` warnings are pre-existing (from third-party shadcn/ui packages) and unrelated to these changes.

---

## No concerns.
