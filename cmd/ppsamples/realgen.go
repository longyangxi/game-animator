package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"perfectpixel/internal/config"
	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// Generates real sprites through the actual OpenRouter (gemini-3-pro-image) pipeline.
// It uses an AI-drawn magenta key strip as the comparison input, not a synthetic drawing.

const (
	demoDesc  = "a brave knight in blue steel armor with a flowing red cape and a round shield"
	demoState = "walk"
	demoN     = 6
)

func provider() (gen.Provider, string, error) {
	s := config.Load()
	cfg := s.Cfg(s.Provider)
	if cfg.APIKey == "" {
		return nil, "", fmt.Errorf("no %s API key", s.Provider)
	}
	p, err := gen.New(s.Provider, cfg.APIKey, cfg.Model)
	return p, s.Provider, err
}

func decodePNGorJPEG(raw []byte) (*image.NRGBA, error) {
	im, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(im), nil
}

// buildRealBase actually generates the base character and returns it as magenta RGBA.
// (Input for the matting & pixelization demos. The segmentation & alignment demos use the real matted strips from sample/.)
func buildRealBase() (*image.NRGBA, error) {
	// Reuse an already-generated real base if present (saves an API call, identical real output).
	if im, err := loadPNG(outDir + "/real-base.png"); err == nil {
		fmt.Println("  reusing existing real-base.png")
		return im, nil
	}
	p, name, err := provider()
	if err != nil {
		return nil, err
	}
	fmt.Printf("  provider=%s generating base character for real...\n", name)
	style := sprite.ResolveStyle("pixel", "")

	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	bp := sprite.BuildCharacterPrompt(demoDesc, style, "")
	t0 := time.Now()
	braw, err := p.GenerateImage(ctx, bp, nil, "1:1")
	if err != nil {
		return nil, fmt.Errorf("base generation failed: %w", err)
	}
	base, err := decodePNGorJPEG(braw)
	if err != nil {
		return nil, fmt.Errorf("base decode failed: %w", err)
	}
	fmt.Printf("  base generated %dx%d (%.0fs)\n", base.Rect.Dx(), base.Rect.Dy(), time.Since(t0).Seconds())
	save("real-base.png", base)
	return base, nil
}

var _ = os.Stdout
var _ = demoState
