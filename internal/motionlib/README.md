# motionlib — pose-template library

`templates/<preset>.png` are 8-frame **front-row** strips used as a generation
reference image to guide animation motion (see `sprite.PoseTemplateClause`).

## ⚠️ Validation-phase placeholders

These strips are cropped from `dreiachse-cyber/image-cockpit-for-codex-workflows`
(`public/samples/*-sheet.png`, license marked `sample`). They are used **only**
as an internal generation input and never appear in user output. **Replace them
with our own validated art before any public release.**

Regenerate with: `go run ./internal/motionlib/cropfront.go`

## Source sheet -> preset mapping

| image-cockpit sheet | preset |
|---|---|
| idle-breathing | idle |
| walk-cycle | walk |
| run-cycle | run |
| basic-attack | attack |
| hurt-reaction | hurt |
| death-downed | death |
| spell-cast | cast |
| jump-hop | jump |
| victory-cheer | cheer |
| knockback | knockback |
| guard-block | block |
