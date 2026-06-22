package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// runGen runs the full pipeline: base generation -> per-state generation -> bundle export -> summary output.
func runGen(opt options) error {
	p, provider, model, err := resolveProvider(opt)
	if err != nil {
		return err
	}
	var presets []sprite.PresetInfo
	if !opt.baseOnly {
		presets, err = selectStates(opt)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(opt.out, 0o755); err != nil {
		return err
	}

	logf := func(format string, a ...any) {
		if !opt.quiet && !opt.jsonOut {
			fmt.Printf(format, a...)
		}
	}
	logf("provider: %s · model: %s · style: %s · %d states · output: %s\n",
		provider, model, opt.style, len(presets), opt.out)

	ctx, cancel := context.WithTimeout(context.Background(), opt.timeout)
	defer cancel()

	style := sprite.ResolveStyle(opt.style, "")

	// 1) Base character
	logf("generating base character... ")
	t0 := time.Now()
	baseClean, baseBytes, err := generateBase(ctx, p, opt.desc, opt.style, style)
	if err != nil {
		return fmt.Errorf("base generation failed: %w", err)
	}
	savePNG(filepath.Join(opt.out, "base.png"), baseClean)
	logf("done (%.0fs)\n", time.Since(t0).Seconds())

	// baseonly: skip state/bundle generation and keep only base.png.
	if opt.baseOnly {
		if opt.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"provider": provider, "model": model, "style": opt.style,
				"base": filepath.Join(opt.out, "base.png"),
			})
		}
		logf("baseonly done · %s\n", filepath.Join(opt.out, "base.png"))
		return nil
	}

	// 2) Per-state generation
	var states []sprite.StateFrames
	var rows []resultRow
	for _, kw := range presets {
		spec := sprite.StateSpec{Name: kw.Name, Frames: kw.Frames, FPS: kw.FPS, Loop: kw.Loop, Action: kw.Action}
		ts := time.Now()
		logf("[%s] generating %s... ", kw.Category, kw.Name)
		res := genState(ctx, p, opt, style, spec, [][]byte{baseBytes}, baseClean)
		logf("%d/%d found, %d attempts, score %d (%.0fs)\n", res.Found, res.Expected, res.Attempts, res.Score, time.Since(ts).Seconds())
		rows = append(rows, res.row())
		if len(res.frames) > 0 {
			states = append(states, sprite.StateFrames{Spec: spec, Frames: res.frames})
		}
	}

	// 3) 8-direction set (optional)
	if strings.TrimSpace(opt.dirset) != "" {
		logf("=== 8-direction set: %s ===\n", opt.dirset)
		dirStates, dirRows := genDirectionSet(ctx, p, opt, style, opt.dirset, baseBytes, baseClean, logf)
		states = append(states, dirStates...)
		rows = append(rows, dirRows...)
	}

	if len(states) == 0 {
		return fmt.Errorf("no states were generated, so there is no bundle to export")
	}

	// 4) Export the game-engine bundle
	summary, err := exportBundle(opt.out, opt.desc, states, rows)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	summary.Provider = provider
	summary.Model = model
	summary.Style = opt.style

	if opt.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	logf("\nbundle done · %d states · sheet %dx%d · %s\n",
		summary.Animations, summary.SheetWidth, summary.SheetHeight, opt.out)
	logf("  - sprite-sheet.png / manifest.json / sprite-sheet.json (Aseprite)\n")
	logf("  - frames/<state>/frame-NN.png · gif/<state>.gif · apng/<state>.png\n")
	return nil
}

// genDirectionSet builds an 8-direction set from 5 AI-generated directions plus 3 mirrored directions.
func genDirectionSet(ctx context.Context, p gen.Provider, opt options, style, key string,
	baseBytes []byte, baseClean *image.NRGBA, logf func(string, ...any)) ([]sprite.StateFrames, []resultRow) {

	pre, ok := sprite.PresetByName(key)
	if !ok {
		logf("8-direction set: unknown keyword %q (skipping)\n", key)
		return nil, nil
	}
	var states []sprite.StateFrames
	var rows []resultRow
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
		logf("  [%s] generating... ", d)
		res := genState(ctx, p, opt, style, spec, refs, bN)
		logf("%d/%d found, score %d\n", res.Found, res.Expected, res.Score)
		rows = append(rows, res.row())
		if len(res.frames) > 0 {
			states = append(states, sprite.StateFrames{Spec: spec, Frames: res.frames})
			frameByDir[d] = res.frames
			if d == "south" && res.rawClean != nil {
				southRef = pngBytes(res.rawClean)
			}
		}
	}

	// Mirrored directions: west<-east, south-west<-south-east, north-west<-north-east
	mirror := map[string]string{"west": "east", "south-west": "south-east", "north-west": "north-east"}
	for dst, src := range mirror {
		srcFrames := frameByDir[src]
		if len(srcFrames) == 0 {
			continue
		}
		var mirrored []*image.NRGBA
		for _, f := range srcFrames {
			mirrored = append(mirrored, sprite.MirrorNRGBA(f))
		}
		spec := sprite.StateSpec{Name: key + "-" + dst, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action, Facing: dst}
		states = append(states, sprite.StateFrames{Spec: spec, Frames: mirrored})
		rows = append(rows, resultRow{Name: spec.Name, Expected: pre.Frames, Found: len(mirrored), Status: "mirrored"})
		logf("  [%s] mirrored(%s) %d frames\n", dst, src, len(mirrored))
	}
	return states, rows
}
