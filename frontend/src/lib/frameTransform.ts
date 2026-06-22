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

// Renders the transformed frame onto a fresh cellSize canvas and returns a PNG dataURL.
// Identity transform returns the source PNG unchanged (fast path).
export async function bakeTransformed(
  png: string,
  t: FrameTransform | undefined,
  cellSize: number,
): Promise<string> {
  if (isIdentity(t)) return png;
  // assumes the source PNG is cell-sized (frames are generated at cell resolution)
  const img = await loadImage(png);
  const canvas = document.createElement("canvas");
  canvas.width = cellSize;
  canvas.height = cellSize;
  const ctx = canvas.getContext("2d");
  if (!ctx) return png;
  applyTransform(ctx, img, img.width, img.height, t, cellSize);
  return canvas.toDataURL("image/png");
}
