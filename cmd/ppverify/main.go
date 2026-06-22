// Command ppverify re-runs the current (new) pipeline as-is over the 100 real
// strips in sample/ without any AI calls, quantitatively measuring quality and
// comparing it against the past baseline (sample/report.json).
// Purpose: verify whether algorithm improvements actually help on real data.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"

	"perfectpixel/internal/sprite"
)

type baseline struct {
	PassRate float64 `json:"passRate"`
	AvgScore float64 `json:"avgScore"`
	Results  []struct {
		Name     string  `json:"name"`
		Expected int     `json:"expected"`
		Found    int     `json:"found"`
		Score    int     `json:"score"`
		Identity float64 `json:"identity"`
		Motion   float64 `json:"motion"`
	} `json:"results"`
}

type row struct {
	path                               string
	expected, found                    int
	identity, motion, contact, overall float64
	errs                               int
}

func loadPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(img), nil
}

func countFrames(dir string) int {
	m, _ := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	return len(m)
}

func main() {
	strips, _ := filepath.Glob("sample/*/*/_strip.png")
	sort.Strings(strips)
	if len(strips) == 0 {
		fmt.Println("no sample strips found (run from repo root)")
		os.Exit(1)
	}

	var rows []row
	var sumId, sumMo, sumCo, sumOv float64
	hit := 0
	var regress []row

	for _, sp := range strips {
		dir := filepath.Dir(sp)
		expected := countFrames(dir)
		if expected == 0 {
			continue
		}
		nimg, err := loadPNG(sp)
		if err != nil {
			fmt.Printf("decode fail %s: %v\n", sp, err)
			continue
		}
		// _strip.png is already a background-removed transparent strip, so feed it
		// straight into ExtractFrames without re-running RemoveBackground (same as ppsamples scan).
		ext := sprite.ExtractFrames(nimg, expected, 256, 256, 16)
		insp := sprite.InspectFrames(ext.Frames, [3]uint8{255, 0, 255}, nil)
		sc := sprite.ScoreFrames(ext.Frames)

		r := row{
			path: dir, expected: expected, found: ext.Found,
			identity: sc.Identity, motion: sc.Motion,
			contact: sc.Contact, overall: sc.Overall, errs: len(insp.Errors),
		}
		rows = append(rows, r)
		sumId += sc.Identity
		sumMo += sc.Motion
		sumCo += sc.Contact
		sumOv += sc.Overall
		if ext.Found == expected {
			hit++
		} else {
			regress = append(regress, r)
		}
	}

	n := float64(len(rows))
	fmt.Printf("\n=== NEW pipeline on %d real sample strips ===\n", len(rows))
	fmt.Printf("frame-count accuracy : %d/%d (%.1f%%)\n", hit, len(rows), 100*float64(hit)/n)
	fmt.Printf("mean identity        : %.3f\n", sumId/n)
	fmt.Printf("mean motion          : %.3f\n", sumMo/n)
	fmt.Printf("mean contact (new)   : %.3f\n", sumCo/n)
	fmt.Printf("mean overall (new)   : %.3f\n", sumOv/n)

	// Baseline comparison
	if bf, err := os.ReadFile("sample/report.json"); err == nil {
		var bl baseline
		if json.Unmarshal(bf, &bl) == nil && len(bl.Results) > 0 {
			var oId, oMo float64
			oHit := 0
			for _, r := range bl.Results {
				oId += r.Identity
				oMo += r.Motion
				if r.Found == r.Expected {
					oHit++
				}
			}
			bn := float64(len(bl.Results))
			fmt.Printf("\n=== OLD baseline (sample/report.json) ===\n")
			fmt.Printf("frame-count accuracy : %d/%d (%.1f%%)\n", oHit, len(bl.Results), 100*float64(oHit)/bn)
			fmt.Printf("mean identity (dHash): %.3f\n", oId/bn)
			fmt.Printf("mean motion          : %.3f\n", oMo/bn)
			fmt.Printf("avg score / passRate : %.1f / %.1f%%\n", bl.AvgScore, bl.PassRate)
		}
	}

	if len(regress) > 0 {
		fmt.Printf("\n=== frame-count mismatches (NEW) %d ===\n", len(regress))
		for _, r := range regress {
			fmt.Printf("  %-48s expected=%d found=%d errs=%d\n", r.path, r.expected, r.found, r.errs)
		}
	}

	// Regression guard: exit with failure if new-pipeline frame accuracy is below 85%
	if float64(hit)/n < 0.85 {
		fmt.Printf("\nFAIL: frame accuracy below 85%%\n")
		os.Exit(1)
	}
	fmt.Printf("\nPASS\n")
}
