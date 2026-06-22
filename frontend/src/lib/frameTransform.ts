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
