// Command ppslice runs the slicing/normalization pipeline offline on raw,
// pre-slice strips exported from the app (the "Strips" button → raw-strips/).
// It consumes NO generation tokens: it only re-slices PNGs already on disk.
//
// For each strip it writes the extracted frames + GIF + APNG, and prints a
// per-frame jitter report — the spread of each frame's alpha centroid and
// content size across the animation. Large spread is the visible symptom of
// the scale-pulse (B1) and centroid-sway (B2) bugs documented in
// docs/sprite-slicing-stability.md. This is our before/after measuring stick.
//
// Usage:
//
//	go run ./cmd/ppslice -in /path/to/raw-strips [-out <dir>] [-cell 256] [-margin 24] [-fps 10]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"

	"perfectpixel/internal/sprite"
)

type stripEntry struct {
	File     string `json:"file"`
	Name     string `json:"name"`
	Expected int    `json:"expected"`
}

func main() {
	var (
		in     = flag.String("in", "", "input dir containing strips.json + *.png (the app's raw-strips/ folder)")
		out    = flag.String("out", "", "output dir (default: <in>/sliced)")
		cell   = flag.Int("cell", 256, "output cell size (square)")
		margin = flag.Int("margin", 24, "safe margin inside each cell")
		fps    = flag.Int("fps", 10, "frames per second for the GIF/APNG preview")
	)
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "ppslice: -in is required (point it at the exported raw-strips/ folder)")
		os.Exit(2)
	}
	outDir := *out
	if outDir == "" {
		outDir = filepath.Join(*in, "sliced")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	entries, err := loadEntries(*in)
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ppslice: no strips found")
		os.Exit(1)
	}

	for _, e := range entries {
		if err := processStrip(*in, outDir, e, *cell, *margin, *fps); err != nil {
			fmt.Printf("  ✗ %s: %v\n", e.File, err)
		}
	}
}

// loadEntries reads strips.json if present, else falls back to every *.png in the dir.
func loadEntries(in string) ([]stripEntry, error) {
	manifestPath := filepath.Join(in, "strips.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var entries []stripEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("strips.json: %w", err)
		}
		return entries, nil
	}
	// fallback: glob PNGs, unknown expected → 0 (honest pose count reported as-is)
	matches, err := filepath.Glob(filepath.Join(in, "*.png"))
	if err != nil {
		return nil, err
	}
	var entries []stripEntry
	for _, m := range matches {
		entries = append(entries, stripEntry{File: filepath.Base(m), Name: filepath.Base(m), Expected: 0})
	}
	return entries, nil
}

func processStrip(in, outDir string, e stripEntry, cell, margin, fps int) error {
	img, err := loadPNG(filepath.Join(in, e.File))
	if err != nil {
		return err
	}
	strip := sprite.ToNRGBA(img)

	res := sprite.ExtractFrames(strip, e.Expected, cell, cell, margin)
	if len(res.Frames) == 0 {
		return fmt.Errorf("no frames extracted (warnings: %v)", res.Warnings)
	}

	stateDir := filepath.Join(outDir, sanitize(e.Name))
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	for i, f := range res.Frames {
		if err := writePNG(filepath.Join(stateDir, fmt.Sprintf("frame-%02d.png", i)), f); err != nil {
			return err
		}
	}
	if gifBytes, err := sprite.EncodeGIF(res.Frames, fps, true); err == nil {
		_ = os.WriteFile(filepath.Join(stateDir, "anim.gif"), gifBytes, 0o644)
	}
	if apngBytes, err := sprite.EncodeAPNG(res.Frames, fps, true); err == nil {
		_ = os.WriteFile(filepath.Join(stateDir, "anim.png"), apngBytes, 0o644)
	}

	rep := jitterReport(res.Frames)
	flag := ""
	if e.Expected > 0 && res.Found != e.Expected {
		flag = fmt.Sprintf("  ⚠ pose count %d≠%d", res.Found, e.Expected)
	}
	fmt.Printf("✓ %-20s frames=%d%s\n", e.Name, len(res.Frames), flag)
	fmt.Printf("    centroid jitter: cxΔ=%.1f cyΔ=%.1f px   size pulse: wΔ=%d hΔ=%d px  → %s\n",
		rep.cxSpread, rep.cySpread, rep.wSpread, rep.hSpread, stateDir)
	return nil
}

type report struct {
	cxSpread, cySpread float64
	wSpread, hSpread   int
}

// jitterReport measures, across all output frames, the spread (max−min) of each
// frame's alpha centroid (cx,cy) and content bbox size. For a stable animation
// the body should not pulse or slide; large spread reveals B1/B2.
func jitterReport(frames []*image.NRGBA) report {
	var cxs, cys []float64
	var ws, hs []int
	for _, f := range frames {
		cx, cy, w, h, ok := centroidAndExtent(f)
		if !ok {
			continue
		}
		cxs = append(cxs, cx)
		cys = append(cys, cy)
		ws = append(ws, w)
		hs = append(hs, h)
	}
	return report{
		cxSpread: spreadF(cxs),
		cySpread: spreadF(cys),
		wSpread:  spreadI(ws),
		hSpread:  spreadI(hs),
	}
}

func centroidAndExtent(img *image.NRGBA) (cx, cy float64, w, h int, ok bool) {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Dx(), b.Dy(), -1, -1
	var sx, sy, sw float64
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := float64(img.Pix[img.PixOffset(x, y)+3])
			if a <= 10 {
				continue
			}
			sx += float64(x) * a
			sy += float64(y) * a
			sw += a
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if sw == 0 {
		return 0, 0, 0, 0, false
	}
	return sx / sw, sy / sw, maxX - minX + 1, maxY - minY + 1, true
}

func spreadF(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	lo, hi := v[0], v[0]
	for _, x := range v {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	return hi - lo
}

func spreadI(v []int) int {
	if len(v) == 0 {
		return 0
	}
	sort.Ints(v)
	return v[len(v)-1] - v[0]
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "strip"
	}
	return string(out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ppslice:", err)
	os.Exit(1)
}
