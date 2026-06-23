import type { FrameTransform } from "../types";

export const SCALE_MIN = 0.5;
export const SCALE_MAX = 1.5;

// Fraction of the cell added as a transparent working margin on EACH side, so frames
// whose content reaches the cell edge (e.g. an extended sword) still have room to be
// nudged in the Align modal without clipping. Tunable; framePad rounds it to whole px.
export const PAD_FRAC = 0.15;

// Pixel margin added per side for a given cell. The padded working canvas is
// cellSize + 2 * framePad(cellSize).
export function framePad(cellSize: number): number {
  return Math.round(cellSize * PAD_FRAC);
}

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
// `pad` shifts the result by a constant (pad, pad), placing the inner cell inset by `pad`
// on every side of a cellSize + 2*pad working canvas. The constant offset is identical for
// every frame, so it never changes content's position relative to other frames (no jitter).
export function computeDrawRect(
  imgW: number,
  imgH: number,
  t: FrameTransform | undefined,
  cellSize: number,
  pad = 0,
): DrawRect {
  const s = t ? t.scale : 1;
  const dx = t ? t.dx : 0;
  const dy = t ? t.dy : 0;
  const w = imgW * s;
  const h = imgH * s;
  const x = cellSize / 2 - w / 2 + dx + pad;
  const y = cellSize - h + dy + pad;
  return { x, y, w, h };
}

export function applyTransform(
  ctx: CanvasRenderingContext2D,
  img: CanvasImageSource,
  imgW: number,
  imgH: number,
  t: FrameTransform | undefined,
  cellSize: number,
  pad = 0,
): void {
  const r = computeDrawRect(imgW, imgH, t, cellSize, pad);
  ctx.imageSmoothingEnabled = false;
  ctx.drawImage(img, r.x, r.y, r.w, r.h);
}

// Convert a transform whose dx/dy are in cell pixels into one for a canvas scaled by k (view px / cell px).
export function scaleTransform(t: FrameTransform | undefined, k: number): FrameTransform | undefined {
  if (!t) return undefined;
  return { scale: t.scale, dx: t.dx * k, dy: t.dy * k };
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = src;
  });
}

// Renders the transformed frame onto a fresh (cellSize + 2*pad) canvas and returns a PNG
// dataURL. With pad > 0 the content is composited with the same (pad, pad) offset used
// everywhere else, so overflow that the user dragged past the original cell edge survives
// to export. The identity fast-path only applies when there is no padding to add.
export async function bakeTransformed(
  png: string,
  t: FrameTransform | undefined,
  cellSize: number,
  pad = 0,
): Promise<string> {
  if (pad === 0 && isIdentity(t)) return png;
  // assumes the source PNG is cell-sized (frames are generated at cell resolution)
  const img = await loadImage(png);
  const size = cellSize + 2 * pad;
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) return png;
  applyTransform(ctx, img, img.width, img.height, t, cellSize, pad);
  return canvas.toDataURL("image/png");
}
