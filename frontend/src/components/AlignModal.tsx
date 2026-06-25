import { useCallback, useEffect, useRef, useState } from "react";
import { Dialog, DialogContent } from "./ui/dialog";
import { Button } from "./ui/button";
import { useI18n } from "../i18n";
import { FrameItem, FrameTransform } from "../types";
import { identityTransform, isIdentity, clampScale, applyTransform, scaleTransform, framePad, outlineSilhouette, PAD_FRAC_MAX } from "../lib/frameTransform";

interface IProps {
  items: FrameItem[];
  index: number;
  cellSize: number;
  padFrac: number; // project-wide working margin (shared by every frame); 0 = none
  onPadFracChange: (frac: number) => void;
  onSave: (t: FrameTransform) => void;
  onClose: () => void;
}

const VIEW = 360; // on-screen canvas size (px)
// Onion-skin direction cues (animation convention): the previous frame is tinted warm (red)
// and the next frame cool (blue), so the user can tell past from future at a glance.
const PREV_COLOR = "#ff4d4d";
const NEXT_COLOR = "#3b9dff";

export default function AlignModal({ items, index, cellSize, padFrac, onPadFracChange, onSave, onClose }: IProps) {
  const { t } = useI18n();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const imgsRef = useRef<Record<number, HTMLImageElement>>({});
  const edgesRef = useRef<Record<number, HTMLCanvasElement>>({}); // colored direction outlines, cached per neighbor
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
    // The on-screen canvas shows the padded working area (cellSize + 2*pad); the original
    // cell is drawn as an inset guide so dragged-out content stays visible and editable.
    const pad = framePad(cellSize, padFrac);
    const paddedCell = cellSize + 2 * pad;
    const k = VIEW / paddedCell;
    const cs = cellSize * k; // inner cell size in view px
    const pk = pad * k;      // pad in view px
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, VIEW, VIEW);
    // padded working-area border (faint) + original cell guide (inset by pad)
    ctx.strokeStyle = "rgba(120,120,255,0.25)";
    ctx.strokeRect(0.5, 0.5, VIEW - 1, VIEW - 1);
    ctx.strokeStyle = "rgba(120,120,255,0.6)";
    ctx.strokeRect(pk + 0.5, pk + 0.5, cs - 1, cs - 1);
    // onion ghosts: faint full-color image keeps interior detail, a colored outline
    // (red = prev / blue = next) marks the direction.
    if (onion) {
      neighbors.forEach((i) => {
        const im = imgsRef.current[i];
        if (!im || !im.complete) return;
        const st = scaleTransform(items[i].transform, k);
        ctx.globalAlpha = 0.3;
        applyTransform(ctx, im, im.width * k, im.height * k, st, cs, pk);
        let edge = edgesRef.current[i];
        if (!edge) {
          edge = outlineSilhouette(im, i < index ? PREV_COLOR : NEXT_COLOR);
          edgesRef.current[i] = edge;
        }
        ctx.globalAlpha = 0.95;
        applyTransform(ctx, edge, im.width * k, im.height * k, st, cs, pk);
      });
      ctx.globalAlpha = 1;
    }
    // current frame with the draft transform (scaled into VIEW space)
    const im = imgsRef.current[index];
    if (im && im.complete) {
      applyTransform(ctx, im, im.width * k, im.height * k, scaleTransform(draftRef.current, k), cs, pk);
    }
  }, [cellSize, padFrac, index, items, neighbors, onion]);

  useEffect(draw, [draw, draft]);

  // drag to move (dx/dy in cell pixels)
  const dragRef = useRef<{ x: number; y: number } | null>(null);
  const onPointerDown = (e: React.PointerEvent) => {
    dragRef.current = { x: e.clientX, y: e.clientY };
    (e.target as Element).setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: React.PointerEvent) => {
    if (!dragRef.current) return;
    const k = (cellSize + 2 * framePad(cellSize, padFrac)) / VIEW; // view px → content px (padded canvas)
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
      if ((e.target as HTMLElement)?.tagName === "INPUT") return;
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
            {onion && (
              <span style={{ display: "flex", gap: 10, marginLeft: 6, fontSize: 12, opacity: 0.85 }}>
                {index > 0 && <LegendDot color={PREV_COLOR} label={t("align_prev")} />}
                {index < items.length - 1 && <LegendDot color={NEXT_COLOR} label={t("align_next")} />}
              </span>
            )}
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
          <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
            {t("align_margin")}
            <input
              type="range" min={0} max={PAD_FRAC_MAX} step={0.01} value={padFrac}
              onChange={(e) => onPadFracChange(+e.target.value)}
              style={{ flex: 1 }}
            />
            <span>{Math.round(padFrac * 100)}%</span>
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

function LegendDot({ color, label }: { color: string; label: string }) {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
      <span style={{ width: 10, height: 10, borderRadius: "50%", background: color, display: "inline-block" }} />
      {label}
    </span>
  );
}
