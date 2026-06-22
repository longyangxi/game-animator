package sprite

import (
	"image"
	"testing"
)

// TestComposeManifestV2 verifies that the composed manifest correctly fills the v2 schema
// (pivot/trim/duration).
func TestComposeManifestV2(t *testing.T) {
	mk := func() *image.NRGBA {
		f := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		fillBox(f, 20, 30, 43, 60, 200, 100, 50) // bottom-center content
		return f
	}
	states := []StateFrames{
		{Spec: StateSpec{Name: "idle", FPS: 8, Loop: true}, Frames: []*image.NRGBA{mk(), mk()}},
	}
	_, m := ComposeAtlas("hero", states, 64, 64)

	if m.Version != 2 || m.Schema == "" || m.Generator == "" {
		t.Fatalf("v2 metadata missing: version=%d schema=%q generator=%q", m.Version, m.Schema, m.Generator)
	}
	a, ok := m.Animations["idle"]
	if !ok {
		t.Fatal("idle animation missing")
	}
	if a.DurationMs != 125 { // 1000/8
		t.Fatalf("durationMs error: %d", a.DurationMs)
	}
	if len(a.Trims) != 2 {
		t.Fatalf("trim count error: %d", len(a.Trims))
	}
	if a.Trims[0].W <= 0 || a.Trims[0].H <= 0 {
		t.Fatalf("trim bbox abnormal: %+v", a.Trims[0])
	}
	// the pivot is the cell horizontal center + the content's lowest point
	if a.Pivot.X != 32 {
		t.Fatalf("pivot.X error: %d", a.Pivot.X)
	}
	if a.Pivot.Y < 55 || a.Pivot.Y > 64 {
		t.Fatalf("pivot.Y (foot anchor) error: %d", a.Pivot.Y)
	}
}
