package sprite

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"perfectpixel/internal/gen"
)

// TestPipelineLive verifies the full pipeline with a real AI (fal): strip generation →
// background removal → frame extraction. It runs only when PP_LIVE_TEST=1 + FAL_KEY are set.
func TestPipelineLive(t *testing.T) {
	if os.Getenv("PP_LIVE_TEST") != "1" {
		t.Skip("runs only when PP_LIVE_TEST=1 is set")
	}
	key := os.Getenv("FAL_KEY")
	if key == "" {
		t.Skip("FAL_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c := gen.NewFal(key, "")

	outDir := filepath.Join(os.TempDir(), "perfectpixel-live")
	_ = os.MkdirAll(outDir, 0o755)

	const expected = 4
	spec := StateSpec{Name: "walk", Frames: expected, FPS: 10, Loop: true, Action: "walking in place, side view"}
	desc := "a small knight with silver armor and a blue plume on the helmet"
	style := StylePresets["pixel"]

	// quality-based automatic retry logic, same as the app (up to 3 times)
	feedback := ""
	var found int
	var frames []*image.NRGBA
	var insp InspectResult
	for attempt := 1; attempt <= 3; attempt++ {
		prompt := BuildStripPrompt(desc, style, spec, feedback)
		raw, err := c.GenerateImage(ctx, prompt, nil, AspectForFrames(expected))
		if err != nil {
			t.Fatalf("[attempt %d] strip generation failed: %v", attempt, err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("[attempt %d] PNG decoding failed: %v", attempt, err)
		}
		nimg := ToNRGBA(img)
		bgKey := DetectBackground(nimg)
		clean := RemoveBackground(nimg)
		res := ExtractFrames(clean, expected, 256, 256, 24)
		found = res.Found
		frames = res.Frames
		insp = InspectFrames(frames, bgKey, nil)
		t.Logf("[attempt %d] extracted %d/%d, extraction warnings: %v, quality errors: %v, quality warnings: %v",
			attempt, found, expected, res.Warnings, insp.Errors, insp.Warnings)

		savePNG(t, filepath.Join(outDir, fmt.Sprintf("strip-attempt%d.png", attempt)), clean)
		if found == expected && insp.Ok() {
			break
		}
		var fixes []string
		if found != expected {
			fixes = append(fixes, fmt.Sprintf(
				"IMPORTANT CORRECTION: the previous attempt contained %d separate sprites but EXACTLY %d are required. Redraw with exactly %d clearly separated poses, each fully surrounded by magenta.",
				found, expected, expected))
		}
		fixes = append(fixes, insp.RetryHints...)
		feedback = strings.Join(fixes, "\n")
	}

	if found != expected {
		t.Fatalf("frame count still mismatched after automatic retries: %d/%d (output: %s)", found, expected, outDir)
	}
	if !insp.Ok() {
		t.Fatalf("quality errors still remain after automatic retries: %v (output: %s)", insp.Errors, outDir)
	}
	for i, f := range frames {
		savePNG(t, filepath.Join(outDir, fmt.Sprintf("frame-%02d.png", i)), f)
		// each frame must have enough actual content pixels (at least 1% of the cell)
		solid := 0
		for p := 3; p < len(f.Pix); p += 4 {
			if f.Pix[p] > 128 {
				solid++
			}
		}
		if solid < 256*256/100 {
			t.Fatalf("frame %d has insufficient content: %d pixels", i, solid)
		}
	}
	t.Logf("E2E success: extracted %d frames, output: %s", found, outDir)
}

func savePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("PNG encoding failed: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("file save failed: %v", err)
	}
}
