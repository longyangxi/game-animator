package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"

	"perfectpixel/internal/sprite"
)

// loadPNG reads a PNG as NRGBA.
func loadPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(im), nil
}

// countFrames counts frame-*.png files in the directory (the expected frame count).
func countFrames(dir string) int {
	m, _ := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	return len(m)
}

type scanRow struct {
	path      string
	n         int
	found     int
	balWo     float64 // equal-split content imbalance
	balWi     float64
	cenWo     float64 // equal-split (bbox center) centroid std dev
	cenWi     float64 // ExtractFrames (centroid) centroid std dev
	segGain   float64 // segmentation gain = balWo/balWi
	cenGain   float64 // alignment gain = cenWo - cenWi
	cross     int     // number of equal-split lines crossing a character
}

// scanSamples computes before/after metrics over all sample/*/*/_strip.png and
// finds strips with high segmentation/alignment contrast (to select demo cases
// from real AI data).
func scanSamples() {
	strips, _ := filepath.Glob("sample/*/*/_strip.png")
	var rows []scanRow
	for _, p := range strips {
		dir := filepath.Dir(p)
		n := countFrames(dir)
		if n < 2 {
			continue
		}
		strip, err := loadPNG(p)
		if err != nil {
			continue
		}
		wo := equalSplitExtract(strip, n, cell, cell, 16)
		ext := sprite.ExtractFrames(strip, n, cell, cell, 16)
		_, _, rWo := frameBalance(wo)
		_, _, rWi := frameBalance(ext.Frames)
		cWo := centroidSpread(wo)
		cWi := centroidSpread(ext.Frames)
		seg := rWo
		if rWi > 0 {
			seg = rWo / rWi
		}
		cross, _ := crossingLines(strip, n)
		rows = append(rows, scanRow{p, n, ext.Found, rWo, rWi, cWo, cWi, seg, cWo - cWi, cross})
	}
	fmt.Printf("scanned %d strips\n", len(rows))

	sort.Slice(rows, func(i, j int) bool { return rows[i].cross > rows[j].cross })
	fmt.Println("\n== top equal-split lines crossing a character (cross/n-1) ==")
	for i := 0; i < len(rows) && i < 8; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d  cross=%d/%d  cenGain=%.1fpx\n", rel(r.path), r.n, r.cross, r.n-1, r.cenGain)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].segGain > rows[j].segGain })
	fmt.Println("\n== top segmentation contrast (higher balWo/balWi = worse equal-split) ==")
	for i := 0; i < len(rows) && i < 6; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d found=%d  balWo=%.1f balWi=%.1f gain=%.1fx\n",
			rel(r.path), r.n, r.found, r.balWo, r.balWi, r.segGain)
	}

	fmt.Println("\n== merged/missing poses (found != n) or max balWo ==")
	sort.Slice(rows, func(i, j int) bool { return rows[i].balWo > rows[j].balWo })
	for i := 0; i < len(rows) && i < 8; i++ {
		r := rows[i]
		flag := ""
		if r.found != r.n {
			flag = "  <-- found!=n"
		}
		fmt.Printf("  %-40s n=%d found=%d balWo=%.2f%s\n", rel(r.path), r.n, r.found, r.balWo, flag)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].cenGain > rows[j].cenGain })
	fmt.Println("\n== top alignment contrast (higher cenWo - cenWi = bigger centroid gain) ==")
	for i := 0; i < len(rows) && i < 6; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d  cenWo=%.1f cenWi=%.1f gain=%.1fpx\n",
			rel(r.path), r.n, r.cenWo, r.cenWi, r.cenGain)
	}
}

func rel(p string) string {
	parts := filepath.SplitList(p)
	_ = parts
	d := filepath.Dir(p)
	return filepath.Base(filepath.Dir(d)) + "/" + filepath.Base(d)
}
