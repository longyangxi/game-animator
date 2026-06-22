# Task 1 Report: Types + vitest + pure transform math

## Files Modified / Created

### Modified
- `frontend/src/types.ts` — Added `FrameTransform` interface above `FrameItem`; added `transform?: FrameTransform` as the last field of `FrameItem`.
- `frontend/vite.config.ts` — Added `/// <reference types="vitest" />` as the first line; added `test: { environment: "node", include: ["src/**/*.test.ts"] }` inside `defineConfig`.
- `frontend/package.json` — Added `"test": "vitest run"` and `"test:watch": "vitest"` scripts; vitest 0.34.6 added to devDependencies (by npm install).
- `frontend/package-lock.json` — Updated by npm install (48 packages added).

### Created
- `frontend/src/lib/frameTransform.ts` — Pure transform math: `SCALE_MIN`, `SCALE_MAX`, `identityTransform`, `isIdentity`, `clampScale`, `DrawRect` interface, `computeDrawRect`.
- `frontend/src/lib/frameTransform.test.ts` — vitest unit tests for all above exports.

## Test Command and Output

### Step 5 (failing run — before implementation):
```
$ npm test
> vitest run
 FAIL  src/lib/frameTransform.test.ts
Error: [vite-node] Failed to load "./frameTransform" imported from ...frameTransform.test.ts
 Test Files  1 failed (1)
      Tests  no tests
```
Confirmed: fails with "Cannot find module" as expected.

### Step 7 (passing run — after implementation):
```
$ npm test
> vitest run
 ✓ src/lib/frameTransform.test.ts  (7 tests) 2ms
 Test Files  1 passed (1)
      Tests  7 passed (7)
   Start at  14:22:52
   Duration  193ms
```
All 7 tests pass.

## TypeScript Check

```
$ node_modules/.bin/tsc --noEmit
(no output — exits 0)
```

## Deviations / Concerns

None. Implementation matches the plan spec verbatim. The `png` field comment in `FrameItem` was updated from `// dataURL` to `// dataURL — source frame, never mutated` as specified in the plan. vitest 0.34.6 installed without peer-dep errors against vite 3.
