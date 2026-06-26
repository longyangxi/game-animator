//go:build ignore

// cropfront downloads image-cockpit official animation sheets and writes the
// FRONT row (row 0 of 5) of each to internal/motionlib/templates/<preset>.png.
// Run from the repo root: `go run ./internal/motionlib/cropfront.go`
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	_ "image/png"
)

const base = "https://raw.githubusercontent.com/dreiachse-cyber/image-cockpit-for-codex-workflows/main/public/samples/"

// sheet filename (without -sheet.png) -> our preset name
var mapping = map[string]string{
	"idle-breathing": "idle",
	"walk-cycle":     "walk",
	"run-cycle":      "run",
	"basic-attack":   "attack",
	"hurt-reaction":  "hurt",
	"death-downed":   "death",
	"spell-cast":     "cast",
	"jump-hop":       "jump",
	"victory-cheer":  "cheer",
	"knockback":      "knockback",
	"guard-block":    "block",
}

func main() {
	out := filepath.Join("internal", "motionlib", "templates")
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	for src, preset := range mapping {
		url := base + src + "-sheet.png"
		resp, err := http.Get(url)
		if err != nil {
			panic(err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			panic(err)
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			panic(fmt.Errorf("%s: %w", src, err))
		}
		b := img.Bounds()
		rowH := b.Dy() / 5 // sheets are 8 columns x 5 direction rows
		sub := image.NewNRGBA(image.Rect(0, 0, b.Dx(), rowH))
		for y := 0; y < rowH; y++ {
			for x := 0; x < b.Dx(); x++ {
				sub.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		f, err := os.Create(filepath.Join(out, preset+".png"))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, sub); err != nil {
			panic(err)
		}
		f.Close()
		fmt.Printf("wrote %s.png (%dx%d)\n", preset, b.Dx(), rowH)
	}
}
