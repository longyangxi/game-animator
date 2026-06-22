# Task 6 Report: Bake Transformed Frames at Export

## Summary

Implemented Task 6 ("Bake transformed frames at export") from `2026-06-22-onion-skin-frame-align.md` against `frontend/src/App.tsx`.

## Files Changed

- `frontend/src/App.tsx`

## Exact Edits

### 1. Added import (line inserted after the `useI18n` import)

```ts
import { bakeTransformed } from "./lib/frameTransform";
```

### 2. Replaced `states:` mapping in `handleExport`

Before:
```ts
        states: done.map((s) => ({
          name: s.name,
          fps: s.fps,
          loop: s.loop,
          frames: selectedFrames(s).map((f) => f.png),
        })),
```

After:
```ts
        states: await Promise.all(
          done.map(async (s) => ({
            name: s.name,
            fps: s.fps,
            loop: s.loop,
            frames: await Promise.all(
              selectedFrames(s).map((f) => bakeTransformed(f.png, f.transform, cellRef.current)),
            ),
          })),
        ),
```

`handleExport` was already `async` and `cellRef.current` was already used elsewhere in the function (confirmed from reading the file before editing).

## Verification Results

- **tsc --noEmit**: exits 0 (no output, no errors)
- **npm run build**: succeeds — `✓ 1858 modules transformed`, output in `dist/`. Pre-existing `'use client'` warnings from shadcn/ui components present before this task; not introduced by this change.

## Concerns

None. Identity frames short-circuit in `bakeTransformed` (returns source PNG unchanged), so unadjusted exports remain byte-identical to the pre-task behavior. The `handleExport` function was already `async`, so the added `await Promise.all(...)` does not require any signature changes. The `cellRef.current` cell size correctly matches the cell size used throughout the rest of the export payload.
