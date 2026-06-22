# Onion-Skin Per-Frame Align — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user manually scale/position any animation frame with onion-skin ghosts of its neighbors, fixing cases automatic normalization gets wrong (e.g. a crouched pickup frame rendered too big).

**Architecture:** Pure frontend. A `{scale, dx, dy}` transform per `FrameItem`, applied at draw time (nearest-neighbor) in the player/thumbnails/modal, and baked into PNGs only at export. No Go change. Non-destructive: the source frame PNG is never mutated.

**Tech Stack:** React 18 + TypeScript + Vite 3; Radix Dialog; lucide-react icons; vitest (added here) for pure-logic unit tests; HTML Canvas 2D for rendering/baking.

## Global Constraints

- Transform is `{ scale: number; dx: number; dy: number }`; `undefined` == identity (`scale 1, dx 0, dy 0`).
- Scale clamped to `[0.5, 1.5]`. Scale anchor = **bottom-center** of the cell (feet stay grounded).
- All canvas draws use `imageSmoothingEnabled = false` (nearest-neighbor, crisp pixels).
- `dx`/`dy` are in **cell pixels**.
- Existing sessions (no `transform` field) must load unchanged → identity.
- i18n: every user-facing string added via `t("key")` with keys defined in all four catalogs (`ui.en.ts`, `ui.zh.ts`, `ui.ko.ts`, `ui.es.ts`).
- All paths below are relative to the repo root; the frontend lives in `frontend/`.

---

## File Structure

- Create `frontend/src/lib/frameTransform.ts` — pure transform math + canvas helpers.
- Create `frontend/src/lib/frameTransform.test.ts` — vitest unit tests.
- Create `frontend/src/components/AlignModal.tsx` — the align editor modal.
- Modify `frontend/src/types.ts` — add `FrameTransform`, `FrameItem.transform`.
- Modify `frontend/src/components/AnimPlayer.tsx` — apply per-frame transform at draw time.
- Modify `frontend/src/components/PreviewPanel.tsx` — align button + badge + CSS-transform thumbnails; own `AlignModal`; pass transforms to `AnimPlayer`.
- Modify `frontend/src/App.tsx` — bake transformed frames at export.
- Modify `frontend/src/i18n/ui.{en,zh,ko,es}.ts` — strings (folded into the tasks that use them).
- Modify `frontend/vite.config.ts` + `frontend/package.json` — vitest setup (Task 1).

---

## Task 1: Types + vitest + pure transform math

**Files:**
- Modify: `frontend/src/types.ts`
- Modify: `frontend/package.json` (devDeps + scripts)
- Modify: `frontend/vite.config.ts` (vitest config)
- Create: `frontend/src/lib/frameTransform.ts`
- Test: `frontend/src/lib/frameTransform.test.ts`

**Interfaces:**
- Produces: `interface FrameTransform { scale: number; dx: number; dy: number }`; `FrameItem.transform?: FrameTransform`; `identityTransform(): FrameTransform`; `isIdentity(t?: FrameTransform): boolean`; `clampScale(s: number): number`; `interface DrawRect { x: number; y: number; w: number; h: number }`; `computeDrawRect(imgW: number, imgH: number, t: FrameTransform | undefined, cellSize: number): DrawRect`; consts `SCALE_MIN = 0.5`, `SCALE_MAX = 1.5`.

- [ ] **Step 1: Add the type.** In `frontend/src/types.ts`, add above `FrameItem`:

```ts
export interface FrameTransform {
  scale: number; // relative to the frame content; clamped to [0.5, 1.5]
  dx: number;    // cell-pixel horizontal offset
  dy: number;    // cell-pixel vertical offset
}
```

Then add `transform?: FrameTransform;` as the last field of `FrameItem`:

```ts
export interface FrameItem {
  id: string;
  png: string; // dataURL — source frame, never mutated
  selected: boolean;
  transform?: FrameTransform; // undefined == identity
}
```

- [ ] **Step 2: Install vitest.**

Run: `cd frontend && npm install -D vitest@^0.34.6`
Expected: adds vitest to devDependencies, no peer-dep errors (vitest 0.34 supports vite 3).

- [ ] **Step 3: Add test config + script.** In `frontend/vite.config.ts`, add `test` to the config object (add `/// <reference types="vitest" />` as the first line of the file):

```ts
/// <reference types="vitest" />
```

and inside `defineConfig({ ... })` add:

```ts
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
```

In `frontend/package.json` `scripts`, add:

```json
    "test": "vitest run",
    "test:watch": "vitest"
```

- [ ] **Step 4: Write the failing test.** Create `frontend/src/lib/frameTransform.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { identityTransform, isIdentity, clampScale, computeDrawRect, SCALE_MIN, SCALE_MAX } from "./frameTransform";

describe("identityTransform / isIdentity", () => {
  it("identity is scale 1, no offset", () => {
    expect(identityTransform()).toEqual({ scale: 1, dx: 0, dy: 0 });
  });
  it("undefined and identity values are identity", () => {
    expect(isIdentity(undefined)).toBe(true);
    expect(isIdentity({ scale: 1, dx: 0, dy: 0 })).toBe(true);
  });
  it("any non-identity value is not identity", () => {
    expect(isIdentity({ scale: 0.9, dx: 0, dy: 0 })).toBe(false);
    expect(isIdentity({ scale: 1, dx: 2, dy: 0 })).toBe(false);
    expect(isIdentity({ scale: 1, dx: 0, dy: -3 })).toBe(false);
  });
});

describe("clampScale", () => {
  it("clamps to [SCALE_MIN, SCALE_MAX]", () => {
    expect(clampScale(2)).toBe(SCALE_MAX);
    expect(clampScale(0.1)).toBe(SCALE_MIN);
    expect(clampScale(1)).toBe(1);
  });
});

describe("computeDrawRect (bottom-center anchor)", () => {
  it("identity fills the cell", () => {
    expect(computeDrawRect(256, 256, undefined, 256)).toEqual({ x: 0, y: 0, w: 256, h: 256 });
  });
  it("scale shrinks about the bottom-center", () => {
    // 0.5 of 256 = 128; centered horizontally (64..192), bottom-aligned (y=128..256)
    expect(computeDrawRect(256, 256, { scale: 0.5, dx: 0, dy: 0 }, 256)).toEqual({ x: 64, y: 128, w: 128, h: 128 });
  });
  it("offset shifts the rect", () => {
    expect(computeDrawRect(256, 256, { scale: 1, dx: 10, dy: -5 }, 256)).toEqual({ x: 10, y: -5, w: 256, h: 256 });
  });
});
```

- [ ] **Step 5: Run it; verify it fails.**

Run: `cd frontend && npm test`
Expected: FAIL — `Cannot find module './frameTransform'` (file not created yet).

- [ ] **Step 6: Implement.** Create `frontend/src/lib/frameTransform.ts`:

```ts
import type { FrameTransform } from "../types";

export const SCALE_MIN = 0.5;
export const SCALE_MAX = 1.5;

export function identityTransform(): FrameTransform {
  return { scale: 1, dx: 0, dy: 0 };
}

export function isIdentity(t?: FrameTransform): boolean {
  return !t || (t.scale === 1 && t.dx === 0 && t.dy === 0);
}

export function clampScale(s: number): number {
  return Math.min(SCALE_MAX, Math.max(SCALE_MIN, s));
}

export interface DrawRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

// Destination rect for drawing a source image (imgW×imgH, normally the cell-sized frame)
// into a cellSize square: scaled by t.scale about the bottom-center, then offset by dx/dy.
export function computeDrawRect(
  imgW: number,
  imgH: number,
  t: FrameTransform | undefined,
  cellSize: number,
): DrawRect {
  const s = t ? t.scale : 1;
  const dx = t ? t.dx : 0;
  const dy = t ? t.dy : 0;
  const w = imgW * s;
  const h = imgH * s;
  const x = cellSize / 2 - w / 2 + dx;
  const y = cellSize - h + dy;
  return { x, y, w, h };
}
```

- [ ] **Step 7: Run tests; verify pass + tsc.**

Run: `cd frontend && npm test && node_modules/.bin/tsc --noEmit`
Expected: all tests PASS; tsc exits 0.

- [ ] **Step 8: Commit.**

```bash
git add frontend/src/types.ts frontend/src/lib/frameTransform.ts frontend/src/lib/frameTransform.test.ts frontend/vite.config.ts frontend/package.json frontend/package-lock.json
git commit -m "feat(align): FrameTransform type + transform math + vitest"
```

---

## Task 2: Canvas helpers — applyTransform + bakeTransformed

**Files:**
- Modify: `frontend/src/lib/frameTransform.ts`
- Test: `frontend/src/lib/frameTransform.test.ts`

**Interfaces:**
- Consumes: `computeDrawRect`, `isIdentity` (Task 1).
- Produces: `applyTransform(ctx, img, imgW, imgH, t, cellSize): void` (draws with NN); `bakeTransformed(png: string, t: FrameTransform | undefined, cellSize: number): Promise<string>` (returns a PNG dataURL; identity → returns `png` unchanged).

- [ ] **Step 1: Write the failing test.** Append to `frontend/src/lib/frameTransform.test.ts`:

```ts
import { applyTransform } from "./frameTransform";

describe("applyTransform", () => {
  it("disables smoothing and draws at the computed rect", () => {
    const calls: any[] = [];
    const ctx: any = { imageSmoothingEnabled: true, drawImage: (...a: any[]) => calls.push(a) };
    const img: any = {};
    applyTransform(ctx, img, 256, 256, { scale: 0.5, dx: 0, dy: 0 }, 256);
    expect(ctx.imageSmoothingEnabled).toBe(false);
    expect(calls).toHaveLength(1);
    expect(calls[0]).toEqual([img, 64, 128, 128, 128]);
  });
});
```

- [ ] **Step 2: Run it; verify it fails.**

Run: `cd frontend && npm test`
Expected: FAIL — `applyTransform is not a function`.

- [ ] **Step 3: Implement.** Append to `frontend/src/lib/frameTransform.ts`:

```ts
export function applyTransform(
  ctx: CanvasRenderingContext2D,
  img: CanvasImageSource,
  imgW: number,
  imgH: number,
  t: FrameTransform | undefined,
  cellSize: number,
): void {
  const r = computeDrawRect(imgW, imgH, t, cellSize);
  ctx.imageSmoothingEnabled = false;
  ctx.drawImage(img, r.x, r.y, r.w, r.h);
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = src;
  });
}

// Renders the transformed frame onto a fresh cellSize canvas and returns a PNG dataURL.
// Identity transform returns the source PNG unchanged (fast path).
export async function bakeTransformed(
  png: string,
  t: FrameTransform | undefined,
  cellSize: number,
): Promise<string> {
  if (isIdentity(t)) return png;
  const img = await loadImage(png);
  const canvas = document.createElement("canvas");
  canvas.width = cellSize;
  canvas.height = cellSize;
  const ctx = canvas.getContext("2d");
  if (!ctx) return png;
  applyTransform(ctx, img, img.width, img.height, t, cellSize);
  return canvas.toDataURL("image/png");
}
```

- [ ] **Step 4: Run tests; verify pass + tsc.**

Run: `cd frontend && npm test && node_modules/.bin/tsc --noEmit`
Expected: PASS; tsc 0. (`bakeTransformed` is DOM-dependent — covered by manual verification in Task 6, not unit-tested.)

- [ ] **Step 5: Commit.**

```bash
git add frontend/src/lib/frameTransform.ts frontend/src/lib/frameTransform.test.ts
git commit -m "feat(align): applyTransform + bakeTransformed canvas helpers"
```

---

## Task 3: AlignModal component

**Files:**
- Create: `frontend/src/components/AlignModal.tsx`
- Modify: `frontend/src/i18n/ui.en.ts`, `ui.zh.ts`, `ui.ko.ts`, `ui.es.ts`

**Interfaces:**
- Consumes: `FrameTransform`, `FrameItem` (types); `identityTransform`, `isIdentity`, `clampScale`, `applyTransform` (frameTransform).
- Produces: `export default function AlignModal(props: { items: FrameItem[]; index: number; cellSize: number; onSave: (t: FrameTransform) => void; onClose: () => void }): JSX.Element`.

- [ ] **Step 1: Add i18n keys.** In each of `ui.en.ts`, `ui.zh.ts`, `ui.ko.ts`, `ui.es.ts`, add these keys (values per language):

`ui.en.ts`:
```ts
  align_title: "Align frame",
  align_onion: "Onion skin",
  align_reset: "Reset",
  align_done: "Done",
  align_cancel: "Cancel",
  align_scale: "Scale",
  align_hint: "Drag to move · scroll to scale · arrow keys nudge 1px",
  align_open: "Align this frame",
  align_adjusted: "Adjusted",
```
`ui.zh.ts`:
```ts
  align_title: "对齐帧",
  align_onion: "洋葱皮",
  align_reset: "重置",
  align_done: "完成",
  align_cancel: "取消",
  align_scale: "缩放",
  align_hint: "拖动移动 · 滚轮缩放 · 方向键微调 1px",
  align_open: "对齐这一帧",
  align_adjusted: "已调整",
```
`ui.ko.ts`:
```ts
  align_title: "프레임 정렬",
  align_onion: "어니언 스킨",
  align_reset: "초기화",
  align_done: "완료",
  align_cancel: "취소",
  align_scale: "크기",
  align_hint: "드래그로 이동 · 스크롤로 크기 · 방향키 1px 미세조정",
  align_open: "이 프레임 정렬",
  align_adjusted: "조정됨",
```
`ui.es.ts`:
```ts
  align_title: "Alinear fotograma",
  align_onion: "Papel cebolla",
  align_reset: "Restablecer",
  align_done: "Hecho",
  align_cancel: "Cancelar",
  align_scale: "Escala",
  align_hint: "Arrastra para mover · rueda para escalar · flechas 1px",
  align_open: "Alinear este fotograma",
  align_adjusted: "Ajustado",
```

- [ ] **Step 2: Create the component.** Create `frontend/src/components/AlignModal.tsx`:

```tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Dialog, DialogContent } from "./ui/dialog";
import { Button } from "./ui/button";
import { useI18n } from "../i18n";
import { FrameItem, FrameTransform } from "../types";
import { identityTransform, isIdentity, clampScale, applyTransform } from "../lib/frameTransform";

interface IProps {
  items: FrameItem[];
  index: number;
  cellSize: number;
  onSave: (t: FrameTransform) => void;
  onClose: () => void;
}

const VIEW = 360; // on-screen canvas size (px)

export default function AlignModal({ items, index, cellSize, onSave, onClose }: IProps) {
  const { t } = useI18n();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const imgsRef = useRef<Record<number, HTMLImageElement>>({});
  const [onion, setOnion] = useState(true);
  const [draft, setDraft] = useState<FrameTransform>(items[index].transform ?? identityTransform());
  const draftRef = useRef(draft);
  draftRef.current = draft;

  const neighbors = [index - 1, index + 1].filter((i) => i >= 0 && i < items.length);

  // load current + neighbor images once
  useEffect(() => {
    const want = [index, ...neighbors];
    want.forEach((i) => {
      if (imgsRef.current[i]) return;
      const img = new Image();
      img.onload = () => draw();
      img.src = items[i].png;
      imgsRef.current[i] = img;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index]);

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const scale = VIEW / cellSize;
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, VIEW, VIEW);
    // cell border + baseline guide
    ctx.strokeStyle = "rgba(120,120,255,0.5)";
    ctx.strokeRect(0.5, 0.5, VIEW - 1, VIEW - 1);
    // onion ghosts (neighbors at identity)
    if (onion) {
      ctx.globalAlpha = 0.25;
      neighbors.forEach((i) => {
        const im = imgsRef.current[i];
        if (im && im.complete) ctx.drawImage(im, 0, 0, VIEW, VIEW);
      });
      ctx.globalAlpha = 1;
    }
    // current frame with the draft transform (scaled into VIEW space)
    const im = imgsRef.current[index];
    if (im && im.complete) {
      applyTransform(ctx, im, im.width * scale, im.height * scale, draftRef.current, VIEW);
    }
  }, [cellSize, index, neighbors, onion]);

  useEffect(draw, [draw, draft]);

  // drag to move (dx/dy in cell pixels)
  const dragRef = useRef<{ x: number; y: number } | null>(null);
  const onPointerDown = (e: React.PointerEvent) => {
    dragRef.current = { x: e.clientX, y: e.clientY };
    (e.target as Element).setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: React.PointerEvent) => {
    if (!dragRef.current) return;
    const k = cellSize / VIEW; // view px → cell px
    const ndx = draftRef.current.dx + (e.clientX - dragRef.current.x) * k;
    const ndy = draftRef.current.dy + (e.clientY - dragRef.current.y) * k;
    dragRef.current = { x: e.clientX, y: e.clientY };
    setDraft((d) => ({ ...d, dx: Math.round(ndx), dy: Math.round(ndy) }));
  };
  const onPointerUp = () => (dragRef.current = null);

  const onWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    setDraft((d) => ({ ...d, scale: clampScale(+(d.scale - e.deltaY * 0.001).toFixed(3)) }));
  };

  // arrow-key 1px nudge
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const map: Record<string, [number, number]> = {
        ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1],
      };
      const d = map[e.key];
      if (!d) return;
      e.preventDefault();
      setDraft((s) => ({ ...s, dx: s.dx + d[0], dy: s.dy + d[1] }));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          <strong>{t("align_title")} #{index + 1}</strong>
          <canvas
            ref={canvasRef}
            width={VIEW}
            height={VIEW}
            className="checker"
            style={{ width: VIEW, height: VIEW, touchAction: "none", cursor: "move" }}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onWheel={onWheel}
          />
          <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <input type="checkbox" checked={onion} onChange={(e) => setOnion(e.target.checked)} />
            {t("align_onion")}
          </label>
          <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
            {t("align_scale")}
            <input
              type="range" min={0.5} max={1.5} step={0.01} value={draft.scale}
              onChange={(e) => setDraft((d) => ({ ...d, scale: clampScale(+e.target.value) }))}
              style={{ flex: 1 }}
            />
            <span>{draft.scale.toFixed(2)}×</span>
          </label>
          <span className="hint">{t("align_hint")}</span>
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <Button variant="ghost" size="sm" disabled={isIdentity(draft)} onClick={() => setDraft(identityTransform())}>
              {t("align_reset")}
            </Button>
            <Button variant="ghost" size="sm" onClick={onClose}>{t("align_cancel")}</Button>
            <Button size="sm" onClick={() => onSave(draft)}>{t("align_done")}</Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 3: Type-check.**

Run: `cd frontend && node_modules/.bin/tsc --noEmit`
Expected: exits 0. (If `Dialog`/`DialogContent` import path differs, match `frontend/src/components/ui/dialog.tsx`'s exports — confirm by opening that file.)

- [ ] **Step 4: Commit.**

```bash
git add frontend/src/components/AlignModal.tsx frontend/src/i18n/ui.en.ts frontend/src/i18n/ui.zh.ts frontend/src/i18n/ui.ko.ts frontend/src/i18n/ui.es.ts
git commit -m "feat(align): AlignModal with onion skin, drag/scale/nudge"
```

---

## Task 4: AnimPlayer applies per-frame transforms

**Files:**
- Modify: `frontend/src/components/AnimPlayer.tsx`

**Interfaces:**
- Consumes: `applyTransform` (frameTransform); `FrameTransform` (types).
- Produces: `AnimPlayer` now accepts optional `transforms?: (FrameTransform | undefined)[]` parallel to `frames`.

- [ ] **Step 1: Add the prop.** In `frontend/src/components/AnimPlayer.tsx`, add to the imports:

```ts
import { FrameTransform } from "../types";
import { applyTransform } from "../lib/frameTransform";
```

Add to `IProps`:

```ts
  transforms?: (FrameTransform | undefined)[];
```

Add `transforms` to the destructured props and to the component signature:
`export default function AnimPlayer({ frames, fps, loop, cellSize, transforms }: IProps) {`

- [ ] **Step 2: Keep a live ref to transforms.** Right after `const stateRef = useRef(...)`, add:

```ts
  const transformsRef = useRef(transforms);
  transformsRef.current = transforms;
```

- [ ] **Step 3: Draw through the transform.** In the render `tick`, replace the final draw block:

```ts
      const img = imgs[Math.min(st.idx, imgs.length - 1)];
      if (!img || !img.naturalWidth) return;
      const w = img.naturalWidth;
      const h = img.naturalHeight;
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      const ctx = canvas.getContext("2d")!;
      ctx.imageSmoothingEnabled = false;
      ctx.clearRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0);
```

with:

```ts
      const idx = Math.min(st.idx, imgs.length - 1);
      const img = imgs[idx];
      if (!img || !img.naturalWidth) return;
      const w = img.naturalWidth;
      const h = img.naturalHeight;
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      const ctx = canvas.getContext("2d")!;
      ctx.imageSmoothingEnabled = false;
      ctx.clearRect(0, 0, w, h);
      applyTransform(ctx, img, w, h, transformsRef.current?.[idx], w);
```

(`applyTransform` with cellSize = the image's own pixel width `w` keeps identity frames pixel-identical to today's `drawImage(img,0,0)`.)

- [ ] **Step 4: Type-check.**

Run: `cd frontend && node_modules/.bin/tsc --noEmit`
Expected: exits 0.

- [ ] **Step 5: Commit.**

```bash
git add frontend/src/components/AnimPlayer.tsx
git commit -m "feat(align): AnimPlayer applies per-frame transform at draw time"
```

---

## Task 5: PreviewPanel — align button, badge, thumbnail preview, modal wiring

**Files:**
- Modify: `frontend/src/components/PreviewPanel.tsx`

**Interfaces:**
- Consumes: `AlignModal` (Task 3); `FrameTransform`, `FrameItem` (types); `isIdentity`, `computeDrawRect` (frameTransform); `onUpdateState` (existing prop that merges a partial `StateDef`).
- Produces: no new exports; updates `state.items[i].transform` via `onUpdateState`.

- [ ] **Step 1: Imports + state.** In `frontend/src/components/PreviewPanel.tsx`:
  - Add to the lucide-react import: `Move` (used as the align icon).
  - Add imports:

```ts
import { useState } from "react"; // if not already imported; otherwise add useState to the existing import
import AlignModal from "./AlignModal";
import { FrameTransform } from "../types";
import { isIdentity } from "../lib/frameTransform";
```

  - Inside the component body, add: `const [alignIdx, setAlignIdx] = useState<number | null>(null);`

- [ ] **Step 2: Pass transforms to the player.** Find `const frames = selectedFrames(state).map((f) => f.png);` and add below it:

```ts
  const frameTransforms = selectedFrames(state).map((f) => f.transform);
```

Then where `<AnimPlayer ... frames={frames} ...>` is rendered, add the prop `transforms={frameTransforms}`.

- [ ] **Step 3: Add align button + badge + thumbnail preview.** Replace the frame-cell `<img>` and add an align button. Change:

```tsx
                  <img src={f.png} className="pixelated" alt={`frame ${i + 1}`} />
                  <span className="fc-num">#{i + 1}</span>
```

to:

```tsx
                  <img
                    src={f.png}
                    className="pixelated"
                    alt={`frame ${i + 1}`}
                    style={
                      isIdentity(f.transform)
                        ? undefined
                        : {
                            transformOrigin: "bottom center",
                            transform: `translate(${(f.transform!.dx / cellSize) * 100}%, ${(f.transform!.dy / cellSize) * 100}%) scale(${f.transform!.scale})`,
                          }
                    }
                  />
                  <span className="fc-num">#{i + 1}</span>
                  {!isIdentity(f.transform) && <span className="fc-num" style={{ left: "auto", right: 4 }}>{t("align_adjusted")}</span>}
                  <span className="fc-move" style={{ left: 4, right: "auto" }} onClick={(e) => e.stopPropagation()}>
                    <button onClick={() => setAlignIdx(i)} title={t("align_open")}>
                      <Move size={10} />
                    </button>
                  </span>
```

- [ ] **Step 4: Render the modal.** Just before the closing `</>` of the `tab === "frames"` block (after the `frame-grid` div / refine box), add:

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

- [ ] **Step 5: Type-check.**

Run: `cd frontend && node_modules/.bin/tsc --noEmit`
Expected: exits 0.

- [ ] **Step 6: Manual verification.** Run `npm run dev` (or the wails dev app). Open a done state with frames → click the align (Move) icon on a frame → modal opens with onion ghosts → drag/scroll/arrow keys move & scale → Done. Confirm: thumbnail reflects the transform, the player loop reflects it, an "Adjusted" badge appears, Reset clears it.

- [ ] **Step 7: Commit.**

```bash
git add frontend/src/components/PreviewPanel.tsx
git commit -m "feat(align): frame align button, adjusted badge, thumbnail preview, modal wiring"
```

---

## Task 6: Bake transformed frames at export

**Files:**
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `bakeTransformed` (frameTransform); existing `ExportProject`, `selectedFrames`, `cellRef`.

- [ ] **Step 1: Import.** In `frontend/src/App.tsx` add:

```ts
import { bakeTransformed } from "./lib/frameTransform";
```

- [ ] **Step 2: Bake in handleExport.** In `handleExport`, replace the `states:` mapping that builds frame dataURLs. Change:

```ts
        states: done.map((s) => ({
          name: s.name,
          fps: s.fps,
          loop: s.loop,
          frames: selectedFrames(s).map((f) => f.png),
        })),
```

to build baked frames first (note: `ExportProject` call becomes async-mapped):

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

(`handleExport` is already `async`; `bakeTransformed` returns the source PNG untouched for identity frames, so unadjusted exports are byte-identical to today.)

- [ ] **Step 3: Type-check + build.**

Run: `cd frontend && node_modules/.bin/tsc --noEmit && npm run build`
Expected: tsc 0; vite build succeeds.

- [ ] **Step 4: Manual verification.** Adjust a frame (Task 5), then Export → open the exported GIF/APNG/sheet → the adjusted frame reflects the scale/offset; unadjusted states are unchanged.

- [ ] **Step 5: Commit.**

```bash
git add frontend/src/App.tsx
git commit -m "feat(align): bake per-frame transforms into exported frames"
```

---

## Self-Review notes (addressed)

- **Spec coverage:** data model (T1) · transform math + bake (T1/T2) · AlignModal w/ onion skin, drag, scroll, arrow nudge, reset, toggle (T3) · live preview in player (T4) and thumbnails (T5) · adjusted badge + entry button (T5) · export bake (T6) · persistence (T1 — `transform` on `FrameItem`, already serialized by `SaveSession`/`LoadSession`; verified by manual reload in T5/T6). Onion-skin neighbor edge cases handled in `AlignModal` (`neighbors` filter).
- **Out-of-scope** items (rotation/flip, tinted onion skin, configurable depth, re-extraction) intentionally absent.
- **Type consistency:** `FrameTransform {scale,dx,dy}`, `computeDrawRect`/`applyTransform`/`bakeTransformed` signatures, and `AlignModal` props are used identically across tasks.
- **Persistence note:** no code change needed beyond the type — confirm during T5/T6 manual steps that Save/Load round-trips `transform`.
