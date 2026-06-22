package gen

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"testing"
	"time"
)

// TestFalLive is a smoke test that calls the real fal.ai API.
// It runs only when PP_LIVE_TEST=1 and FAL_KEY are set (caution: consumes credits).
func TestFalLive(t *testing.T) {
	if os.Getenv("PP_LIVE_TEST") != "1" {
		t.Skip("runs only when PP_LIVE_TEST=1 is set")
	}
	key := os.Getenv("FAL_KEY")
	if key == "" {
		t.Skip("FAL_KEY is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := NewFal(key, "")
	data, err := c.GenerateImage(ctx,
		"A single small red circle centered on a solid magenta (#FF00FF) background. Flat colors, no gradients.",
		nil, "1:1")
	if err != nil {
		t.Fatalf("fal image generation failed: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PNG decoding failed (len=%d): %v", len(data), err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("image size is abnormally small: %v", b)
	}
	t.Logf("fal generation succeeded: %dx%d (%d bytes)", b.Dx(), b.Dy(), len(data))

	// reference image -> /edit endpoint path verification (the real sprite strip generation path)
	data2, err := c.GenerateImage(ctx,
		"Move the red circle to the left edge. Keep the solid magenta (#FF00FF) background.",
		[][]byte{data}, "1:1")
	if err != nil {
		t.Fatalf("fal edit generation failed: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(data2)); err != nil {
		t.Fatalf("edit PNG decoding failed (len=%d): %v", len(data2), err)
	}
	t.Logf("fal edit succeeded: %d bytes", len(data2))
}
