// Command ppposetest is a one-off experiment harness that tests whether a
// VALIDATED, hand-picked motion sprite-sheet (borrowed from the
// image-cockpit-for-codex-workflows "official animations" library) can be used
// as a POSE TEMPLATE to drive our animation generation — keeping our own
// character's identity, but copying the borrowed sheet's attack choreography.
//
// It generates the SAME action three ways so they can be eyeballed side by side:
//
//	A  pose template + our text choreography  (image2 poses + preset Hint)
//	B  pose template only                     (image2 poses, no text choreography)
//	C  control / baseline                     (our current path: text only, no pose)
//
// Everything else (provider, style contract, extraction, pixel post-process, GIF
// export) reuses the real pipeline so the comparison is honest. Outputs land in
// -out as <variant>-strip.png (raw generation), <variant>.gif, and frame PNGs.
//
// Example:
//
//	OPENAI_API_KEY=sk-... go run ./cmd/ppposetest \
//	  -base experiments/pose-template/assets/our-knight.png \
//	  -pose experiments/pose-template/assets/basic-attack-sheet.png \
//	  -state attack -out experiments/pose-template/out
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"

	"perfectpixel/internal/config"
	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

func main() {
	var (
		basePath  = flag.String("base", "", "path to OUR base character PNG (the identity to animate) — required")
		posePath  = flag.String("pose", "experiments/pose-template/assets/basic-attack-sheet.png", "path to the borrowed pose sprite-sheet PNG")
		poseRows  = flag.Int("poserows", 5, "number of direction rows in the pose sheet (front row is cropped out)")
		poseRow   = flag.Int("poserow", 0, "which row to use as the pose template (0 = front)")
		desc      = flag.String("desc", "", "optional subject notes about OUR character")
		styleKey  = flag.String("style", "pixel", "style key (pixel|chibi|cartoon|retro16)")
		stateName = flag.String("state", "attack", "preset state name to generate")
		outDir    = flag.String("out", "experiments/pose-template/out", "output directory")
		provider  = flag.String("provider", "openai", "provider id")
		key       = flag.String("key", "", "API key override (else config.json / OPENAI_API_KEY env)")
		model     = flag.String("model", "", "model override")
		control   = flag.Bool("control", true, "also generate the text-only no-pose baseline (variant C)")
		timeout   = flag.Duration("timeout", 10*time.Minute, "overall timeout")
	)
	flag.Parse()

	if err := run(*basePath, *posePath, *poseRows, *poseRow, *desc, *styleKey, *stateName,
		*outDir, *provider, *key, *model, *control, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "ppposetest failed: %v\n", err)
		os.Exit(1)
	}
}

func run(basePath, posePath string, poseRows, poseRow int, desc, styleKey, stateName,
	outDir, provider, keyOverride, modelOverride string, control bool, timeout time.Duration) error {

	if basePath == "" {
		return fmt.Errorf("-base is required (a PNG of OUR character to animate)")
	}
	pre, ok := sprite.PresetByName(stateName)
	if !ok {
		return fmt.Errorf("unknown state %q (see ppgen -dump for the catalog)", stateName)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Resolve provider + key (config.json / .env / env vars, with CLI overrides).
	s := config.Load()
	if provider == "" {
		provider = s.Provider
	}
	cfg := s.Cfg(provider)
	key := cfg.APIKey
	if keyOverride != "" {
		key = keyOverride
	}
	model := cfg.Model
	if modelOverride != "" {
		model = modelOverride
	}
	if key == "" {
		return fmt.Errorf("no API key for provider %q (set OPENAI_API_KEY, config.json, or pass -key)", provider)
	}
	p, err := gen.New(provider, key, model)
	if err != nil {
		return err
	}
	if model == "" {
		model = gen.DefaultModelFor(provider)
	}

	// Load OUR character (identity reference, image 1).
	baseBytes, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("read base: %w", err)
	}

	// Load + crop the borrowed pose sheet down to the chosen direction row (image 2).
	poseSheet, err := os.ReadFile(posePath)
	if err != nil {
		return fmt.Errorf("read pose sheet: %w", err)
	}
	poseStrip, err := cropRow(poseSheet, poseRows, poseRow)
	if err != nil {
		return fmt.Errorf("crop pose row: %w", err)
	}
	savePNG(filepath.Join(outDir, "pose-template-strip.png"), poseStrip)

	style := sprite.ResolveStyle(styleKey, "")
	aspect := sprite.AspectForFrames(pre.Frames)
	palette := sprite.PaletteSizeForStyle(styleKey)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	poseBytes := encodePNG(poseStrip)

	// Variant A — pose template + our text choreography.
	specA := sprite.StateSpec{Name: pre.Name, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action}
	promptA := sprite.BuildStripPrompt(desc, style, specA, "", "") + poseTemplateClause(stateName, pre.Frames)

	// Variant B — pose template only (no attack-specific text choreography).
	// A name not in the preset catalog yields an empty MotionHint, so no
	// choreography line leaks in; the movement line defers to the pose image.
	specB := sprite.StateSpec{Name: stateName + "-posetemplate", Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop,
		Action: "perform the action shown in the attached pose template strip"}
	promptB := sprite.BuildStripPrompt(desc, style, specB, "", "") + poseTemplateClause(stateName, pre.Frames)

	type variant struct {
		id     string
		prompt string
		refs   [][]byte
	}
	variants := []variant{
		{"A_pose+text", promptA, [][]byte{baseBytes, poseBytes}},
		{"B_pose-only", promptB, [][]byte{baseBytes, poseBytes}},
	}
	if control {
		// Variant C — our current production path: text only, no pose reference.
		promptC := sprite.BuildStripPrompt(desc, style, specA, "", "")
		variants = append(variants, variant{"C_control-textonly", promptC, [][]byte{baseBytes}})
	}

	fmt.Printf("provider=%s model=%s style=%s state=%s frames=%d → %s\n",
		provider, model, styleKey, stateName, pre.Frames, outDir)

	for _, v := range variants {
		t0 := time.Now()
		fmt.Printf("[%s] generating... ", v.id)
		raw, err := p.GenerateImage(ctx, v.prompt, v.refs, aspect)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		img, err := decodeImg(raw)
		if err != nil {
			fmt.Printf("decode error: %v\n", err)
			continue
		}
		// Save the raw generation strip (pre-extraction) for honest eyeballing.
		savePNG(filepath.Join(outDir, v.id+"-strip.png"), img)

		clean := sprite.RemoveBackground(img)
		ext := sprite.ExtractFrames(clean, pre.Frames, 256, 256, 24)
		sprite.PixelPostProcess(ext.Frames, palette)

		// Per-frame PNGs + GIF.
		fdir := filepath.Join(outDir, v.id+"-frames")
		_ = os.MkdirAll(fdir, 0o755)
		for i, f := range ext.Frames {
			savePNG(filepath.Join(fdir, fmt.Sprintf("frame-%02d.png", i)), f)
		}
		if len(ext.Frames) > 0 {
			if b, err := sprite.EncodeGIF(ext.Frames, pre.FPS, pre.Loop); err == nil {
				_ = os.WriteFile(filepath.Join(outDir, v.id+".gif"), b, 0o644)
			}
		}
		fmt.Printf("%d/%d frames, motion=%.3f (%.0fs)\n",
			ext.Found, pre.Frames, sprite.MotionPresence(ext.Frames), time.Since(t0).Seconds())
	}

	fmt.Printf("\ndone — compare:\n")
	fmt.Printf("  %s/pose-template-strip.png   (borrowed poses fed in)\n", outDir)
	for _, v := range variants {
		fmt.Printf("  %s/%s.gif\n", outDir, v.id)
	}
	return nil
}

// poseTemplateClause is the new instruction that repurposes a DIFFERENT
// character's motion sheet as a pose template: identity comes from image 1,
// only the body choreography comes from image 2, and image 2's effects/colours
// are explicitly discarded.
func poseTemplateClause(stateName string, frames int) string {
	return fmt.Sprintf(`
Pose template (CRITICAL — read carefully): TWO images are attached.
- Image 1 is the CANONICAL CHARACTER. Take ALL identity from it: face, hairstyle, body build, outfit, armor, cape, palette, and the weapon or signature prop. The output MUST be image 1's character.
- Image 2 is a POSE TEMPLATE showing a DIFFERENT character performing the "%s" action across several frames, read left to right. Copy ONLY the body choreography from image 2: stance, limb positions, weight shift, wind-up, strike, and recovery arc. Re-pose OUR character (image 1) through those same key poses and motion timing.
- Do NOT copy image 2's character, colours, outfit, weapon shape, or its background. Do NOT reproduce any glow, slash-arc, swoosh, streak, spark, or motion effect drawn in image 2 — render only OUR character's solid body in those poses on the clean keying background.
- Distribute the template's motion across our EXACTLY %d poses (start/wind-up, peak strike, settle).
`, stateName, frames)
}

// cropRow returns the chosen direction row of a multi-row sprite sheet as its
// own PNG strip (the pose template we actually feed the model).
func cropRow(sheet []byte, rows, row int) (image.Image, error) {
	src, _, err := image.Decode(bytes.NewReader(sheet))
	if err != nil {
		return nil, err
	}
	if rows < 1 {
		rows = 1
	}
	if row < 0 || row >= rows {
		return nil, fmt.Errorf("row %d out of range [0,%d)", row, rows)
	}
	b := src.Bounds()
	rowH := b.Dy() / rows
	y0 := b.Min.Y + row*rowH
	sub := image.NewNRGBA(image.Rect(0, 0, b.Dx(), rowH))
	for y := 0; y < rowH; y++ {
		for x := 0; x < b.Dx(); x++ {
			sub.Set(x, y, src.At(b.Min.X+x, y0+y))
		}
	}
	return sub, nil
}

func decodeImg(raw []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func savePNG(path string, img image.Image) {
	if b := encodePNG(img); len(b) > 0 {
		_ = os.WriteFile(path, b, 0o644)
	}
}
