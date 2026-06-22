# Task 5 Report: PreviewPanel — align button, badge, thumbnail preview, modal wiring

## Summary

All four steps from Task 5 were applied to `frontend/src/components/PreviewPanel.tsx`. tsc exits 0 and `npm run build` succeeds.

---

## Exact Edits Made

### Step 1: Imports + state

**Import changes** (lines 2-4, now lines 2-6):
- Added `Move` to the lucide-react import.
- Added `FrameTransform` to the `../types` import.
- Added two new imports: `AlignModal from "./AlignModal"` and `{ isIdentity } from "../lib/frameTransform"`.

**State** added inside component body after `[feedback, setFeedback]`:
```ts
const [alignIdx, setAlignIdx] = useState<number | null>(null);
```

### Step 2: frameTransforms + AnimPlayer prop

Added below `const frames = selectedFrames(state).map((f) => f.png);`:
```ts
const frameTransforms = selectedFrames(state).map((f) => f.transform);
```

**AnimPlayer call-site change**: Added `transforms={frameTransforms}` to the existing `<AnimPlayer ... />` at the `tab === "play"` render site:
```tsx
<AnimPlayer frames={frames} fps={state.fps} loop={state.loop} cellSize={cellSize} transforms={frameTransforms} />
```

### Step 3: img style + badge + align button

Replaced the bare `<img src={f.png} ... />` in the frame-grid map with a styled version carrying CSS transform, added an "Adjusted" badge, and added a new `fc-move` span for the Move/align button. The existing `fc-move` span for ChevronLeft/ChevronRight reorder buttons was kept unchanged. The new align button span uses `style={{ left: 4, right: "auto" }}` to differentiate from the reorder buttons span (no positional style override).

### Step 4: AlignModal render

Added inside the `tab === "frames"` fragment, after the `{state.rawStrip && ...}` block and before the closing `</>`:
```tsx
{alignIdx !== null && (
  <AlignModal
    items={state.items}
    index={alignIdx}
    cellSize={cellSize}
    onClose={() => setAlignIdx(null)}
    onSave={(tr: FrameTransform) => {
      onUpdateState(state.id, {
        items: state.items.map((f, i) => (i === alignIdx ? { ...f, transform: tr } : f)),
      });
      setAlignIdx(null);
    }}
  />
)}
```

---

## tsc + Build Results

- `cd frontend && node_modules/.bin/tsc --noEmit` — **exits 0, no output**
- `cd frontend && npm run build` — **succeeds**: 1858 modules transformed, dist assets written. Pre-existing `'use client' was ignored` warnings from Radix UI components; not introduced by this task and not errors.

---

## Concerns / Deviations

**Dual fc-move spans**: The real file already had a `<span className="fc-move">` for the reorder buttons (ChevronLeft/ChevronRight). The spec says to replace the existing block, but that would remove the reorder buttons. Instead, the align button was added as a separate `fc-move` span with `style={{ left: 4, right: "auto" }}` positioning while the existing reorder span was preserved. This keeps all functionality intact. If the plan's intent was to collapse both into one span, that would require a CSS/layout decision not specified in the plan.
