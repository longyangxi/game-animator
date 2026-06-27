package sprite

import (
	"image"
	"testing"
)

func TestPingPongFrames(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	b := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	c := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	d := image.NewNRGBA(image.Rect(0, 0, 1, 1))

	// 3 -> 4: A B C B
	got := PingPongFrames([]*image.NRGBA{a, b, c})
	want := []*image.NRGBA{a, b, c, b}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d: got %p, want %p", i, got[i], want[i])
		}
	}

	// 4 -> 6: A B C D C B
	got4 := PingPongFrames([]*image.NRGBA{a, b, c, d})
	want4 := []*image.NRGBA{a, b, c, d, c, b}
	if len(got4) != len(want4) {
		t.Fatalf("len = %d, want %d", len(got4), len(want4))
	}
	for i := range want4 {
		if got4[i] != want4[i] {
			t.Errorf("frame %d: got %p, want %p", i, got4[i], want4[i])
		}
	}

	// fewer than 3 frames: returned unchanged
	two := []*image.NRGBA{a, b}
	if got := PingPongFrames(two); len(got) != 2 {
		t.Errorf("len<3 must be unchanged, got len %d", len(got))
	}
}

func TestIsPingPong(t *testing.T) {
	if !IsPingPong("idle") {
		t.Error("idle should be ping-pong")
	}
	if !IsPingPong("meditate-south") { // direction suffix stripped
		t.Error("meditate-south should resolve to ping-pong meditate")
	}
	if IsPingPong("walk") {
		t.Error("walk (locomotion) must NOT be ping-pong")
	}
	if IsPingPong("attack") {
		t.Error("attack must NOT be ping-pong")
	}
}
