package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"perfectpixel/internal/sprite"
)

// resultRow is the per-state generation quality summary (one row in the summary JSON).
type resultRow struct {
	Name     string   `json:"name"`
	Expected int      `json:"expected"`
	Found    int      `json:"found"`
	Attempts int      `json:"attempts,omitempty"`
	Score    int      `json:"score"`
	Identity float64  `json:"identity"`
	Motion   float64  `json:"motion"`
	Contact  float64  `json:"contact"`
	Status   string   `json:"status"`
	Errors   []string `json:"errors,omitempty"`
}

// exportSummary is the stdout JSON structure that machine-readably summarizes a ppgen run.
type exportSummary struct {
	OK          bool        `json:"ok"`
	OutDir      string      `json:"outDir"`
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	Style       string      `json:"style"`
	Character   string      `json:"character"`
	Animations  int         `json:"animations"`
	SheetWidth  int         `json:"sheetWidth"`
	SheetHeight int         `json:"sheetHeight"`
	Files       []string    `json:"files"`
	Results     []resultRow `json:"results"`
}

// exportBundle writes the per-state frames to disk as a game-engine bundle.
// It produces the same artifacts as the installable app's ExportProject (directly to outDir, without a dialog).
func exportBundle(outDir, character string, states []sprite.StateFrames, rows []resultRow) (exportSummary, error) {
	const cell = 256
	sheet, manifest := sprite.ComposeAtlas(character, states, cell, cell)

	files := []string{"base.png"}

	// 1) Sprite sheet PNG
	sheetPath := filepath.Join(outDir, "sprite-sheet.png")
	savePNG(sheetPath, sheet)
	files = append(files, "sprite-sheet.png")

	// 2) PerfectPixel runtime manifest
	if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), data, 0o644); err != nil {
			return exportSummary{}, err
		}
		files = append(files, "manifest.json")
	}

	// 3) Aseprite-compatible sheet JSON (Phaser/Unity/Godot import)
	if data, err := sprite.BuildAsepriteJSON(manifest); err == nil {
		if err := os.WriteFile(filepath.Join(outDir, "sprite-sheet.json"), data, 0o644); err != nil {
			return exportSummary{}, err
		}
		files = append(files, "sprite-sheet.json")
	}

	// 4) Per-state frame PNGs + GIF + APNG
	framesRoot := filepath.Join(outDir, "frames")
	gifRoot := filepath.Join(outDir, "gif")
	apngRoot := filepath.Join(outDir, "apng")
	for _, st := range states {
		name := st.Spec.Name
		dir := filepath.Join(framesRoot, name)
		_ = os.MkdirAll(dir, 0o755)
		for i, f := range st.Frames {
			savePNG(filepath.Join(dir, fmt.Sprintf("frame-%02d.png", i)), f)
		}
		fps := st.Spec.FPS
		if fps <= 0 {
			fps = 8
		}
		if len(st.Frames) > 0 {
			_ = os.MkdirAll(gifRoot, 0o755)
			if b, err := sprite.EncodeGIF(st.Frames, fps, st.Spec.Loop); err == nil {
				_ = os.WriteFile(filepath.Join(gifRoot, name+".gif"), b, 0o644)
			}
			_ = os.MkdirAll(apngRoot, 0o755)
			if b, err := sprite.EncodeAPNG(st.Frames, fps, st.Spec.Loop); err == nil {
				_ = os.WriteFile(filepath.Join(apngRoot, name+".png"), b, 0o644)
			}
		}
	}
	files = append(files, "frames/", "gif/", "apng/")

	return exportSummary{
		OK:          true,
		OutDir:      outDir,
		Character:   character,
		Animations:  len(manifest.Animations),
		SheetWidth:  manifest.Sheet.Width,
		SheetHeight: manifest.Sheet.Height,
		Files:       files,
		Results:     rows,
	}, nil
}
