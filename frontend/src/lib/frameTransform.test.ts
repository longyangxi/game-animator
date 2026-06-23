import { describe, it, expect } from "vitest";
import { identityTransform, isIdentity, clampScale, computeDrawRect, framePad, PAD_FRAC_MAX, SCALE_MIN, SCALE_MAX } from "./frameTransform";

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
  it("pad shifts the rect into the padded canvas by (pad, pad)", () => {
    // identity content, padded canvas: the inner cell sits inset by pad on every side
    expect(computeDrawRect(256, 256, undefined, 256, 10)).toEqual({ x: 10, y: 10, w: 256, h: 256 });
  });
  it("pad composes with scale+offset (constant +pad on x and y)", () => {
    // base (scale 0.5): {x:64,y:128,w:128,h:128}; +pad 20 → {x:84,y:148}
    expect(computeDrawRect(256, 256, { scale: 0.5, dx: 0, dy: 0 }, 256, 20)).toEqual({ x: 84, y: 148, w: 128, h: 128 });
  });
  it("pad defaults to 0 (regression: identical to unpadded)", () => {
    expect(computeDrawRect(256, 256, { scale: 1, dx: 10, dy: -5 }, 256, 0))
      .toEqual(computeDrawRect(256, 256, { scale: 1, dx: 10, dy: -5 }, 256));
  });
});

describe("framePad", () => {
  it("rounds cellSize * frac to a whole-pixel margin per side", () => {
    expect(framePad(256, 0.15)).toBe(38); // round(38.4)
    expect(framePad(200, 0.1)).toBe(20);
    expect(Number.isInteger(framePad(256, 0.15))).toBe(true);
  });
  it("defaults to no margin (frac defaults to 0)", () => {
    expect(framePad(256)).toBe(0);
    expect(framePad(256, 0)).toBe(0);
    expect(framePad(0, 0.15)).toBe(0);
  });
  it("exposes a slider upper bound", () => {
    expect(PAD_FRAC_MAX).toBeGreaterThan(0);
  });
});

import { applyTransform, scaleTransform } from "./frameTransform";

describe("scaleTransform", () => {
  it("scales dx/dy by k, leaves scale", () => {
    expect(scaleTransform({ scale: 0.5, dx: 10, dy: -4 }, 1.5)).toEqual({ scale: 0.5, dx: 15, dy: -6 });
  });
  it("passes undefined through", () => {
    expect(scaleTransform(undefined, 2)).toBeUndefined();
  });
});

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
  it("draws at the pad-offset rect when pad is given", () => {
    const calls: any[] = [];
    const ctx: any = { imageSmoothingEnabled: true, drawImage: (...a: any[]) => calls.push(a) };
    const img: any = {};
    applyTransform(ctx, img, 256, 256, { scale: 1, dx: 0, dy: 0 }, 256, 10);
    expect(calls[0]).toEqual([img, 10, 10, 256, 256]);
  });
});
