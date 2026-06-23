# 2D/2.5D Perspective Toggle — execution ledger

Plan: docs/superpowers/plans/2026-06-23-2d-25d-perspective-toggle.md
Mode: subagent-driven. BASE before Task 1 = 41f34c2.

- Task 1: complete (commits 41f34c2..c18656d, golden snapshot, review clean)
- Task 2: complete (commits c18656d..6c65416, perspective param threaded, flat byte-identical, review clean). Minor: perspectiveStripClause iso branch ends \n\n (intentional per brief, cosmetic).
- Task 3: complete (commits 6c65416..7368199, iso smoke test, review clean)
- Task 4: complete (commits 7368199..2379106, backend Perspective args field, review clean)
- Task 5: complete (commits 2379106..0eca2c0, CharacterDef.perspective + 3 sites + load fallback, tsc clean, review clean)
- Task 6: complete (commits 0eca2c0..4620fa1, View Select + both generate calls + i18n x4, tsc+build clean, review clean). Manual GUI smoke pending human.
- Final whole-branch review (opus): MERGE WITH FIXES — Critical: 6 cmd/ call sites (ppgen/ppvalidate/ppsamples) not updated for new builder signatures → `go build ./...` broken. Root cause: plan only enumerated internal/+app.go callers (grep was scoped to those), missing cmd/. Design otherwise excellent (losslessness byte-identical, iso isolated, key/type consistent E2E).
- Fix wave (commit 0be24e0): appended `, ""` to all 6 cmd/ call sites. Controller-verified: `go build ./...` exit 0; TestPromptGoldenFlat+TestPromptIso pass without -update; 49/49 sprite tests green.
- FEATURE COMPLETE & VERIFIED. Outstanding: GUI visual smoke test (iso renders ¾ tilt) pending human. Minors (non-blocking): perspectiveStripClause \n\n (cosmetic); `as any` casts on generate calls (pre-existing).
