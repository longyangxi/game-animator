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
