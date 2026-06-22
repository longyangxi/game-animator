// Command ppvalidate is a headless harness that drives the real AI generation
// pipeline without a GUI to validate sprite quality.
// It faithfully reproduces the app's GenerateState logic
// (prompt -> generate -> background removal -> frame extraction -> quality check -> pixelation)
// to collect quality scores for generation results across categories and directions.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"
	"path/filepath"
	"strings"
	"time"

	"perfectpixel/internal/config"
	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// stripResult holds the quality measurement result for one state (animation) generation.
type stripResult struct {
	Name     string
	Expected int
	Found    int
	Attempts int
	FPS      int
	Loop     bool
	Score    int
	Identity float64
	Motion   float64
	Contact  float64
	Errors   []string
	Warnings []string
	rel      string // subdirectory relative to outDir (roster mode: char-NN, otherwise empty)
	frames   []*image.NRGBA
	rawClean *image.NRGBA // cleaned strip before quantization (motion reference for direction sets)
}

func (r stripResult) ok() bool { return r.Found == r.Expected && len(r.Errors) == 0 }

func decode(raw []byte) (*image.NRGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(img), nil
}

func savePNG(path string, img image.Image) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		fmt.Printf("[save error] PNG encode %s: %v\n", path, err)
		return
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fmt.Printf("[save error] write %s: %v\n", path, err)
	}
}

// maxAttempts is the maximum number of quality-correction regeneration attempts per state (-attempts flag).
var maxAttempts = 3

// genStrip generates one state using the same automatic retry quality-correction loop as the app.
func genStrip(ctx context.Context, p gen.Provider, desc, styleKey, style string,
	spec sprite.StateSpec, refs [][]byte, baseN *image.NRGBA) (stripResult, error) {

	expected := spec.Frames
	aspect := sprite.AspectForFrames(expected)
	palette := sprite.PaletteSizeForStyle(styleKey)
	feedback := ""

	var best stripResult
	bestScore := -1 << 30
	best.Name, best.Expected = spec.Name, expected

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		prompt := sprite.BuildStripPrompt(desc, style, spec, feedback)
		if len(refs) > 1 {
			prompt += "\nMotion reference: the second attached image is the FRONT-view animation strip of this same character performing this exact action. Reproduce the same motion timing and pose phases frame by frame, but viewed from the required facing direction above.\n"
		}
		raw, err := p.GenerateImage(ctx, prompt, refs, aspect)
		if err != nil {
			return best, err
		}
		nimg, err := decode(raw)
		if err != nil {
			continue
		}
		bgKey := sprite.DetectBackground(nimg)
		clean := sprite.RemoveBackground(nimg)
		ext := sprite.ExtractFrames(clean, expected, 256, 256, 24)
		insp := sprite.InspectFrames(ext.Frames, bgKey, baseN)
		sprite.PixelPostProcess(ext.Frames, palette)

		cand := stripResult{
			Name: spec.Name, Expected: expected, Found: ext.Found, Attempts: attempt,
			FPS: spec.FPS, Loop: spec.Loop, Motion: sprite.MotionPresence(ext.Frames),
			frames: ext.Frames, rawClean: clean,
		}
		cand.Warnings = append(cand.Warnings, ext.Warnings...)
		cand.Warnings = append(cand.Warnings, insp.Warnings...)
		cand.Errors = append(cand.Errors, insp.Errors...)

		if cand.ok() {
			qm := sprite.ScoreFrames(ext.Frames)
			cand.Score = int(qm.Overall * 100)
			cand.Identity = qm.Identity
			cand.Motion = qm.Motion
			cand.Contact = qm.Contact
			return cand, nil
		}
		score := cand.Found*100 - len(cand.Errors)*10
		if score > bestScore {
			best, bestScore = cand, score
		}

		var fixes []string
		if cand.Found != expected {
			fixes = append(fixes, fmt.Sprintf(
				"IMPORTANT CORRECTION: the last attempt read as %d poses but EXACTLY %d are required. Redraw as %d equal columns, one clearly separated pose per column, each ringed by a clean magenta gutter.",
				cand.Found, expected, expected))
		}
		fixes = append(fixes, insp.RetryHints...)
		feedback = strings.Join(fixes, "\n")
	}
	if len(best.frames) > 0 {
		qm := sprite.ScoreFrames(best.frames)
		best.Score = int(qm.Overall * 100)
		best.Identity = qm.Identity
		best.Motion = qm.Motion
		best.Contact = qm.Contact
	}
	return best, nil
}

func main() {
	var (
		percat    = flag.Int("percat", 1, "number of sample keywords per category (0 = base only)")
		listFlag  = flag.String("keywords", "", "comma-separated list of specific keywords (overrides percat when set)")
		dirset    = flag.String("dirset", "", "keyword to generate an 8-direction set for (empty = skip)")
		outDir    = flag.String("out", filepath.Join(os.TempDir(), "ppvalidate"), "output directory")
		desc      = flag.String("desc", "a small knight with silver armor and a blue plume on the helmet", "character description")
		styleKey  = flag.String("style", "pixel", "style key")
		timeout   = flag.Duration("timeout", 30*time.Minute, "overall timeout")
		roster    = flag.Int("roster", 0, "roster mode: number of characters to generate (varied characters x states x directions)")
		statesPer = flag.Int("statesper", 5, "roster mode: number of states per character")
		batch     = flag.Int("batch", 10, "roster mode: number of characters to generate concurrently per batch")
		attempts  = flag.Int("attempts", 3, "maximum number of quality-correction regeneration attempts per state")
	)
	dump := flag.Bool("dump", false, "print the preset+direction catalog as JSON and exit (mock data for UI validation)")
	flag.Parse()
	if *attempts >= 1 {
		maxAttempts = *attempts
	}

	if *dump {
		out := map[string]any{
			"presets":    sprite.ListPresets(),
			"directions": sprite.ListDirections(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}

	if *roster > 0 {
		runRosterMode(*roster, *statesPer, *batch, *outDir, *timeout)
		return
	}
	run(*percat, *listFlag, *dirset, *outDir, *desc, *styleKey, *timeout)
}

// runRosterMode sets up the provider and runs the roster (character diversity) validation.
func runRosterMode(chars, statesPer, batch int, outDir string, timeout time.Duration) {
	s := config.Load()
	cfg := s.Cfg(s.Provider)
	if cfg.APIKey == "" {
		fmt.Printf("no key: no API key configured for provider %s\n", s.Provider)
		os.Exit(1)
	}
	p, err := gen.New(s.Provider, cfg.APIKey, cfg.Model)
	if err != nil {
		fmt.Printf("failed to create provider: %v\n", err)
		os.Exit(1)
	}
	model := cfg.Model
	if model == "" {
		model = gen.DefaultModelFor(s.Provider)
	}
	fmt.Printf("roster mode · provider: %s · model: %s · %d characters x %d states = %d samples · batch %d concurrent · pixel style · output: %s\n",
		s.Provider, model, chars, statesPer, chars*statesPer, batch, outDir)
	_ = os.MkdirAll(outDir, 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := runRoster(ctx, p, chars, statesPer, batch, outDir)
	report(outDir, results)
	writeGallery(outDir, results)
}

func pngBytes(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// selectKeywords picks the keywords to validate.
func selectKeywords(percat int, list string) []sprite.PresetInfo {
	byName := map[string]sprite.PresetInfo{}
	for _, p := range sprite.Presets {
		byName[p.Name] = p
	}
	if strings.TrimSpace(list) != "" {
		var out []sprite.PresetInfo
		for _, n := range strings.Split(list, ",") {
			if p, ok := byName[strings.TrimSpace(n)]; ok {
				out = append(out, p)
			}
		}
		return out
	}
	if percat <= 0 {
		return nil
	}
	count := map[string]int{}
	var out []sprite.PresetInfo
	for _, p := range sprite.Presets {
		if count[p.Category] < percat {
			out = append(out, p)
			count[p.Category]++
		}
	}
	return out
}

func run(percat int, list, dirset, outDir, desc, styleKey string, timeout time.Duration) {
	s := config.Load()
	cfg := s.Cfg(s.Provider)
	if cfg.APIKey == "" {
		fmt.Printf("no key: no API key configured for provider %s\n", s.Provider)
		os.Exit(1)
	}
	p, err := gen.New(s.Provider, cfg.APIKey, cfg.Model)
	if err != nil {
		fmt.Printf("failed to create provider: %v\n", err)
		os.Exit(1)
	}
	model := cfg.Model
	if model == "" {
		model = gen.DefaultModelFor(s.Provider)
	}
	fmt.Printf("provider: %s · model: %s · output: %s\n", s.Provider, model, outDir)
	_ = os.MkdirAll(outDir, 0o755)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	style := sprite.ResolveStyle(styleKey, "")
	palette := sprite.PaletteSizeForStyle(styleKey)

	// 1) Generate the base character (also validates the key)
	t0 := time.Now()
	fmt.Print("generating base character... ")
	craw, err := p.GenerateImage(ctx, sprite.BuildCharacterPrompt(desc, style), nil, "1:1")
	if err != nil {
		fmt.Printf("failed: %v\n", err)
		os.Exit(1)
	}
	cimg, err := decode(craw)
	if err != nil {
		fmt.Printf("decode failed: %v\n", err)
		os.Exit(1)
	}
	baseClean := sprite.RemoveBackground(cimg)
	if palette > 0 {
		single := []*image.NRGBA{baseClean}
		sprite.PixelPostProcess(single, palette)
		baseClean = single[0]
	}
	savePNG(filepath.Join(outDir, "base.png"), baseClean)
	baseBytes := pngBytes(baseClean)
	fmt.Printf("done (%.0fs)\n", time.Since(t0).Seconds())

	var results []stripResult

	// 2) Generate single-direction keyword samples
	for _, kw := range selectKeywords(percat, list) {
		spec := sprite.StateSpec{Name: kw.Name, Frames: kw.Frames, FPS: kw.FPS, Loop: kw.Loop, Action: kw.Action}
		ts := time.Now()
		fmt.Printf("[%s] generating %s... ", kw.Category, kw.Name)
		res, err := genStrip(ctx, p, desc, styleKey, style, spec, [][]byte{baseBytes}, baseClean)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			results = append(results, stripResult{Name: kw.Name, Expected: kw.Frames, Errors: []string{err.Error()}})
			continue
		}
		saveFrames(outDir, res)
		fmt.Printf("%d/%d frames attempt%d %s (%.0fs)\n", res.Found, res.Expected, res.Attempts, status(res), time.Since(ts).Seconds())
		results = append(results, res)
	}

	// 3) 8-direction set (if specified)
	if strings.TrimSpace(dirset) != "" {
		results = append(results, genDirectionSet(ctx, p, desc, styleKey, style, dirset, baseBytes, baseClean, outDir)...)
	}

	report(outDir, results)
	writeGallery(outDir, results)
}

// writeGallery builds a static HTML gallery at outDir/index.html for human visual review.
// It lists each state's animation GIF + frame strip + quality score as a card.
func writeGallery(outDir string, results []stripResult) {
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=utf-8><title>PerfectPixel QA</title>")
	b.WriteString("<style>body{background:#16181d;color:#e6e6e6;font:14px system-ui;margin:24px}")
	b.WriteString("h1{font-size:18px}.grid{display:flex;flex-wrap:wrap;gap:16px}")
	b.WriteString(".card{background:#23262e;border-radius:10px;padding:12px;width:230px}")
	b.WriteString(".card img{image-rendering:pixelated;background:#0d0e11 url('data:image/svg+xml;utf8,<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"16\" height=\"16\"><rect width=\"8\" height=\"8\" fill=\"%23222\"/><rect x=\"8\" y=\"8\" width=\"8\" height=\"8\" fill=\"%23222\"/></svg>');border-radius:6px;max-width:206px}")
	b.WriteString(".s{font-weight:700}.ex{color:#7ee787}.go{color:#9ecbff}.fa{color:#e3b341}.po{color:#ff7b72}</style>")
	fmt.Fprintf(&b, "<h1>PerfectPixel Sprite QA · %d</h1><div class=grid>", len(results))
	cls := func(s int) string {
		switch {
		case s >= 85:
			return "ex"
		case s >= 70:
			return "go"
		case s >= 50:
			return "fa"
		default:
			return "po"
		}
	}
	for _, r := range results {
		gif := filepath.Join(r.rel, r.Name, r.Name+".gif")
		b.WriteString("<div class=card>")
		fmt.Fprintf(&b, "<div><b>%s</b></div>", r.Name)
		fmt.Fprintf(&b, "<img src=\"%s\" alt=\"%s\"><div>", gif, r.Name)
		fmt.Fprintf(&b, "<span class='s %s'>score %d</span> · %d/%d · identity %.0f%% · motion %.1f%%</div>",
			cls(r.Score), r.Score, r.Found, r.Expected, r.Identity*100, r.Motion*100)
		if len(r.Errors) > 0 {
			fmt.Fprintf(&b, "<div class=po>%s</div>", strings.Join(r.Errors, "; "))
		}
		b.WriteString("</div>")
	}
	b.WriteString("</div>")
	_ = os.WriteFile(filepath.Join(outDir, "index.html"), []byte(b.String()), 0o644)
	fmt.Printf("gallery: %s\n", filepath.Join(outDir, "index.html"))
}

func status(r stripResult) string {
	if r.ok() {
		return "OK"
	}
	if r.Found != r.Expected {
		return "frame count mismatch"
	}
	return "quality issue"
}

func saveFrames(outDir string, r stripResult) {
	dir := filepath.Join(outDir, r.Name)
	_ = os.MkdirAll(dir, 0o755)
	if r.rawClean != nil {
		savePNG(filepath.Join(dir, "_strip.png"), r.rawClean)
	}
	for i, f := range r.frames {
		savePNG(filepath.Join(dir, fmt.Sprintf("frame-%02d.png", i)), f)
	}
	// Also save an animation GIF so a human can review motion smoothness
	if len(r.frames) > 0 {
		fps := r.FPS
		if fps <= 0 {
			fps = 8
		}
		if gifBytes, err := sprite.EncodeGIF(r.frames, fps, r.Loop); err == nil {
			_ = os.WriteFile(filepath.Join(dir, r.Name+".gif"), gifBytes, 0o644)
		}
	}
}

// genDirectionSet builds an 8-direction set from 5 AI-generated directions + 3 mirrored ones.
func genDirectionSet(ctx context.Context, p gen.Provider, desc, styleKey, style, key string,
	baseBytes []byte, baseClean *image.NRGBA, outDir string) []stripResult {

	pre, ok := sprite.PresetByName(key)
	if !ok {
		fmt.Printf("8-direction set: unknown keyword %q\n", key)
		return nil
	}
	fmt.Printf("=== 8-direction set: %s ===\n", key)
	var out []stripResult
	frameByDir := map[string][]*image.NRGBA{}
	var southRef []byte

	aiDirs := []string{"south", "east", "north", "south-east", "north-east"}
	for _, d := range aiDirs {
		spec := sprite.StateSpec{Name: key + "-" + d, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action, Facing: d}
		refs := [][]byte{baseBytes}
		if d != "south" && southRef != nil {
			refs = append(refs, southRef)
		}
		var bN *image.NRGBA
		if !sprite.IsBackFacing(d) {
			bN = baseClean
		}
		ts := time.Now()
		fmt.Printf("  [%s] generating... ", d)
		res, err := genStrip(ctx, p, desc, styleKey, style, spec, refs, bN)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			out = append(out, stripResult{Name: spec.Name, Expected: pre.Frames, Errors: []string{err.Error()}})
			continue
		}
		saveFrames(outDir, res)
		frameByDir[d] = res.frames
		if d == "south" && res.rawClean != nil {
			southRef = pngBytes(res.rawClean)
		}
		fmt.Printf("%d/%d %s (%.0fs)\n", res.Found, res.Expected, status(res), time.Since(ts).Seconds())
		out = append(out, res)
	}

	// Mirrored directions: west<-east, south-west<-south-east, north-west<-north-east
	mirror := map[string]string{"west": "east", "south-west": "south-east", "north-west": "north-east"}
	for dst, src := range mirror {
		srcFrames := frameByDir[src]
		if len(srcFrames) == 0 {
			continue
		}
		mres := stripResult{Name: key + "-" + dst, Expected: pre.Frames, Found: len(srcFrames), Attempts: 0, FPS: pre.FPS, Loop: pre.Loop}
		for _, f := range srcFrames {
			mres.frames = append(mres.frames, sprite.MirrorNRGBA(f))
		}
		mres.Motion = sprite.MotionPresence(mres.frames)
		saveFrames(outDir, mres)
		fmt.Printf("  [%s] mirrored(%s) %d frames\n", dst, src, len(mres.frames))
		out = append(out, mres)
	}
	return out
}

func report(outDir string, results []stripResult) {
	if len(results) == 0 {
		fmt.Println("\nNo results generated.")
		return
	}
	var pass, frameFail, qualFail, scoreSum int
	fmt.Println("\n=========== Quality Report ===========")
	for _, r := range results {
		fmt.Printf("%-22s %d/%d  score%3d  identity%3.0f%%  motion%4.1f%%  %-12s",
			r.Name, r.Found, r.Expected, r.Score, r.Identity*100, r.Motion*100, status(r))
		if r.Found >= 2 && r.Motion < 0.02 {
			fmt.Print("  ⚠static")
		}
		if len(r.Errors) > 0 {
			fmt.Printf("  errors:%v", r.Errors)
		}
		if len(r.Warnings) > 0 {
			fmt.Printf("  warnings:%d", len(r.Warnings))
		}
		fmt.Println()
		scoreSum += r.Score
		switch {
		case r.ok():
			pass++
		case r.Found != r.Expected:
			frameFail++
		default:
			qualFail++
		}
	}
	fmt.Println("-----------------------------------")
	avgScore := float64(scoreSum) / float64(len(results))
	fmt.Printf("total %d · pass %d · frame count mismatch %d · quality issue %d · pass rate %.0f%% · avg score %.1f\n",
		len(results), pass, frameFail, qualFail, 100*float64(pass)/float64(len(results)), avgScore)
	writeJSONReport(outDir, results, avgScore, pass)
}

// writeJSONReport writes a machine-readable quality report to outDir/report.json.
func writeJSONReport(outDir string, results []stripResult, avgScore float64, pass int) {
	type row struct {
		Name     string   `json:"name"`
		Expected int      `json:"expected"`
		Found    int      `json:"found"`
		Score    int      `json:"score"`
		Identity float64  `json:"identity"`
		Motion   float64  `json:"motion"`
		Attempts int      `json:"attempts"`
		Status   string   `json:"status"`
		Errors   []string `json:"errors,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
	}
	out := struct {
		Total     int     `json:"total"`
		Pass      int     `json:"pass"`
		PassRate  float64 `json:"passRate"`
		AvgScore  float64 `json:"avgScore"`
		Generated string  `json:"generated"`
		Results   []row   `json:"results"`
	}{Total: len(results), Pass: pass, AvgScore: avgScore, Generated: time.Now().Format(time.RFC3339)}
	if len(results) > 0 {
		out.PassRate = 100 * float64(pass) / float64(len(results))
	}
	for _, r := range results {
		out.Results = append(out.Results, row{
			Name: r.Name, Expected: r.Expected, Found: r.Found, Score: r.Score,
			Identity: r.Identity, Motion: r.Motion, Attempts: r.Attempts,
			Status: status(r), Errors: r.Errors, Warnings: r.Warnings,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outDir, "report.json"), data, 0o644)
	fmt.Printf("report saved: %s\n", filepath.Join(outDir, "report.json"))
}
