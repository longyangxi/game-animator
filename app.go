package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"perfectpixel/internal/config"
	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main body of the Wails application.
type App struct {
	ctx context.Context

	genMu      sync.Mutex
	genCancels map[int]context.CancelFunc // cancel functions for in-progress generation jobs (supports parallel batches)
	genSeq     int
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// provider returns the currently active provider client.
func (a *App) provider() (gen.Provider, error) {
	s := config.Load()
	cfg := s.Cfg(s.Provider)
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key is not configured. Please enter it in Settings", gen.ProviderLabel(s.Provider))
	}
	return gen.New(s.Provider, cfg.APIKey, cfg.Model)
}

func (a *App) emit(event string, data any) {
	runtime.EventsEmit(a.ctx, event, data)
}

// genContext creates a cancelable context for generation.
// In parallel batch generation, multiple contexts may be alive at once, so each is
// tracked and a release function is returned for the caller to free it on completion.
func (a *App) genContext() (context.Context, func()) {
	a.genMu.Lock()
	defer a.genMu.Unlock()
	ctx, cancel := context.WithCancel(a.ctx)
	if a.genCancels == nil {
		a.genCancels = make(map[int]context.CancelFunc)
	}
	id := a.genSeq
	a.genSeq++
	a.genCancels[id] = cancel
	release := func() {
		a.genMu.Lock()
		defer a.genMu.Unlock()
		if c, ok := a.genCancels[id]; ok {
			c()
			delete(a.genCancels, id)
		}
	}
	return ctx, release
}

// CancelGeneration cancels all in-progress generation jobs.
func (a *App) CancelGeneration() {
	a.genMu.Lock()
	defer a.genMu.Unlock()
	for id, cancel := range a.genCancels {
		cancel()
		delete(a.genCancels, id)
	}
}

// friendlyErr converts cancellation errors into user-friendly messages.
func friendlyErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("generation was canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("image generation timed out. Please try again in a moment")
	}
	return err
}

// ---------- Settings ----------

// ProviderInfo is the configuration state for a single provider.
type ProviderInfo struct {
	HasKey     bool     `json:"hasKey"`
	KeyPreview string   `json:"keyPreview"`
	Model      string   `json:"model"`
	Models     []string `json:"models"` // list of selectable models (newest model first)
}

// SettingsInfo is the settings state exposed to the frontend.
type SettingsInfo struct {
	Provider  string                  `json:"provider"`
	Providers map[string]ProviderInfo `json:"providers"`
}

func keyPreview(key string) string {
	if len(key) > 8 {
		return key[:4] + "····" + key[len(key)-4:]
	}
	return ""
}

// GetSettings returns the current settings state (the raw key is never exposed).
func (a *App) GetSettings() SettingsInfo {
	s := config.Load()
	info := SettingsInfo{Provider: s.Provider, Providers: map[string]ProviderInfo{}}
	for _, p := range gen.SupportedProviders {
		cfg := s.Cfg(p)
		model := cfg.Model
		if model == "" {
			model = gen.DefaultModelFor(p)
		}
		info.Providers[p] = ProviderInfo{
			HasKey:     cfg.APIKey != "",
			KeyPreview: keyPreview(cfg.APIKey),
			Model:      model,
			Models:     gen.ModelsFor(p),
		}
	}
	return info
}

// SaveProviderKey validates and then saves the API key for a specific provider.
func (a *App) SaveProviderKey(provider, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("please enter an API key")
	}
	c, err := gen.New(provider, key, "")
	if err != nil {
		return err
	}
	if err := c.ValidateKey(a.ctx); err != nil {
		return err
	}
	s := config.Load()
	s.Cfg(provider).APIKey = key
	return config.Save(s)
}

// SaveProviderModel changes a provider's image model (an empty value resets to the default model).
func (a *App) SaveProviderModel(provider, model string) error {
	switch provider {
	case gen.ProviderGemini, gen.ProviderOpenAI, gen.ProviderOpenRouter, gen.ProviderFal, gen.ProviderBytePlus, gen.ProviderReplicate:
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	model = strings.TrimSpace(model)
	if model == gen.DefaultModelFor(provider) {
		model = "" // store the default model as empty so future default changes are followed
	}
	s := config.Load()
	s.Cfg(provider).Model = model
	return config.Save(s)
}

// SetProvider changes the active provider.
func (a *App) SetProvider(provider string) error {
	switch provider {
	case gen.ProviderGemini, gen.ProviderOpenAI, gen.ProviderOpenRouter, gen.ProviderFal, gen.ProviderBytePlus, gen.ProviderReplicate:
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	s := config.Load()
	s.Provider = provider
	return config.Save(s)
}

// ---------- Session save/restore ----------

// SaveSession writes the current work state (JSON) to disk (for restoring on app restart).
func (a *App) SaveSession(data string) error {
	path, err := config.SessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// write to a temp file then swap it in to prevent corruption
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadSession returns the saved work session JSON (empty string if none exists).
func (a *App) LoadSession() string {
	path, err := config.SessionPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// ClearSession deletes the saved work session.
func (a *App) ClearSession() error {
	path, err := config.SessionPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ---------- Image I/O helpers ----------

var dataURLRe = regexp.MustCompile(`^data:image/[a-zA-Z+.-]+;base64,`)

func decodeDataURL(dataURL string) ([]byte, error) {
	m := dataURLRe.FindString(dataURL)
	if m == "" {
		return nil, errors.New("not valid image data")
	}
	return base64.StdEncoding.DecodeString(dataURL[len(m):])
}

func pngDataURL(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	return img, nil
}

// PickImage opens a file selection dialog and returns the chosen image as a dataURL.
func (a *App) PickImage() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select base image",
		Filters: []runtime.FileFilter{
			{DisplayName: "Images (*.png;*.jpg;*.jpeg;*.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // canceled
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	img, err := decodeImage(data)
	if err != nil {
		return "", err
	}
	return pngDataURL(img)
}

// ---------- Generation pipeline ----------

// GenerateCharacterArgs is a text → base character generation request.
type GenerateCharacterArgs struct {
	Description string `json:"description"`
	StyleKey    string `json:"styleKey"`
	StyleCustom string `json:"styleCustom"`
	Perspective string `json:"perspective"` // "" / "flat" = 2D, "iso" = 2.5D
}

// GenerateCharacter generates a base character image from a description alone.
func (a *App) GenerateCharacter(args GenerateCharacterArgs) (string, error) {
	if strings.TrimSpace(args.Description) == "" {
		return "", errors.New("please enter a character description")
	}
	style := sprite.ResolveStyle(args.StyleKey, args.StyleCustom)
	prompt := sprite.BuildCharacterPrompt(args.Description, style, args.Perspective)

	p, err := a.provider()
	if err != nil {
		return "", err
	}
	// clear the progress indicator on exit
	defer a.emit("progress", map[string]any{"phase": "idle", "message": ""})

	a.emit("progress", map[string]any{"phase": "character", "message": "Generating character..."})
	genCtx, releaseGen := a.genContext()
	defer releaseGen()
	raw, err := p.GenerateImage(genCtx, prompt, nil, "1:1")
	if err != nil {
		return "", friendlyErr(err)
	}
	img, err := decodeImage(raw)
	if err != nil {
		return "", err
	}
	// provide a background-removed preview
	clean := sprite.RemoveBackground(img)
	if n := sprite.PaletteSizeForStyle(args.StyleKey); n > 0 {
		single := []*image.NRGBA{clean}
		sprite.PixelPostProcess(single, n)
		clean = single[0]
	}
	saveGalleryPNG("character-"+galleryStamp(), clean)
	return pngDataURL(clean)
}

// GenerateStateArgs is a per-state strip generation request.
type GenerateStateArgs struct {
	BaseImage   string           `json:"baseImage"` // dataURL
	Description string           `json:"description"`
	StyleKey    string           `json:"styleKey"`
	StyleCustom string           `json:"styleCustom"`
	CellSize    int              `json:"cellSize"`
	SafeMargin  int              `json:"safeMargin"`
	Feedback    string           `json:"feedback"`
	RefStrip    string           `json:"refStrip"` // front (south) strip dataURL — used as motion reference when generating directional sets
	State       sprite.StateSpec `json:"state"`
	Perspective string           `json:"perspective"` // "" / "flat" = 2D, "iso" = 2.5D
}

// StateResult is the result of a state generation.
type StateResult struct {
	Name     string             `json:"name"`
	RawStrip string             `json:"rawStrip"`
	Frames   []string           `json:"frames"`
	Expected int                `json:"expected"`
	Found    int                `json:"found"`
	Warnings []string           `json:"warnings"`
	Scores   sprite.ScoreResult `json:"scores"`
}

// GenerateState generates the strip for one state and extracts its frames.
func (a *App) GenerateState(args GenerateStateArgs) (StateResult, error) {
	res := StateResult{Name: args.State.Name, Expected: args.State.Frames}

	if args.State.Frames < 1 || args.State.Frames > 10 {
		return res, errors.New("the frame count must be between 1 and 10")
	}
	baseRaw, err := decodeDataURL(args.BaseImage)
	if err != nil {
		return res, fmt.Errorf("base image error: %w", err)
	}
	cellSize := args.CellSize
	if cellSize <= 0 {
		cellSize = 256
	}
	margin := args.SafeMargin
	if margin <= 0 {
		margin = max(8, cellSize/12)
	}

	style := sprite.ResolveStyle(args.StyleKey, args.StyleCustom)
	aspect := sprite.AspectForFrames(args.State.Frames)

	p, err := a.provider()
	if err != nil {
		return res, err
	}

	// for base-character identity checks (used by InspectFrames only when the background is transparent).
	// back-facing directions have a different color makeup than the front base, causing false positives, so we skip the check.
	var baseN *image.NRGBA
	if !sprite.IsBackFacing(args.State.Facing) {
		if bimg, err := decodeImage(baseRaw); err == nil {
			baseN = sprite.ToNRGBA(bimg)
		}
	}

	// generation reference images: base character + (optional) front strip
	refs := [][]byte{baseRaw}
	if strings.TrimSpace(args.RefStrip) != "" {
		if refRaw, err := decodeDataURL(args.RefStrip); err == nil {
			refs = append(refs, refRaw)
		}
	}

	// automatically retry until the exact frame count is produced (up to 3 times)
	const maxAttempts = 3
	expected := args.State.Frames
	feedback := args.Feedback
	genCtx, releaseGen := a.genContext()
	defer releaseGen()
	var best StateResult
	var bestImgs []*image.NRGBA
	bestScore := -1 << 30
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		prompt := sprite.BuildStripPrompt(args.Description, style, args.State, feedback, args.Perspective)
		if len(refs) > 1 {
			prompt += "\nMotion reference: the second attached image is the FRONT-view animation strip of this same character performing this exact action. Reproduce the same motion timing and pose phases frame by frame, but viewed from the required facing direction above.\n"
		}

		msg := "Generating AI frames..."
		if attempt > 1 {
			msg = fmt.Sprintf("Regenerating to correct frame count... (%d/%d)", attempt, maxAttempts)
		}
		a.emit("progress", map[string]any{"phase": "generate", "state": args.State.Name, "message": msg})

		stripRaw, err := p.GenerateImage(genCtx, prompt, refs, aspect)
		if err != nil {
			lastErr = friendlyErr(err)
			break // no point retrying API errors/cancellations (the client retries on its own)
		}

		a.emit("progress", map[string]any{"phase": "extract", "state": args.State.Name, "message": "Removing background and extracting frames..."})
		stripImg, err := decodeImage(stripRaw)
		if err != nil {
			lastErr = err
			continue
		}
		nimg := sprite.ToNRGBA(stripImg)
		bgKey := sprite.DetectBackground(nimg)
		clean := sprite.RemoveBackground(nimg)

		cand := StateResult{Name: args.State.Name, Expected: expected}
		if rawURL, err := pngDataURL(clean); err == nil {
			cand.RawStrip = rawURL
		}
		extracted := sprite.ExtractFrames(clean, expected, cellSize, cellSize, margin)
		// per-frame quality checks run on the original frames before quantization (quantization
		// can smear out subtle identity differences, reducing drift-detection sensitivity)
		insp := sprite.InspectFrames(extracted.Frames, bgKey, baseN)
		// for pixel-art styles, apply shared-palette quantization + pixel-grid snapping to make it "true" pixel art
		sprite.PixelPostProcess(extracted.Frames, sprite.PaletteSizeForStyle(args.StyleKey))
		cand.Found = extracted.Found
		cand.Warnings = extracted.Warnings
		for _, f := range extracted.Frames {
			u, err := pngDataURL(f)
			if err != nil {
				return res, err
			}
			cand.Frames = append(cand.Frames, u)
		}
		cand.Warnings = append(cand.Warnings, insp.Errors...)
		cand.Warnings = append(cand.Warnings, insp.Warnings...)
		cand.Scores = sprite.ScoreFrames(extracted.Frames)
		// if adjacent frames barely change (effectively static), emit a non-blocking warning.
		// the 0.01 threshold is measurement-based: it still passes intentionally low-motion poses like meditate (~1.5%)
		// and only catches the "animation doesn't move at all" defect where frames below 0.01 are virtually identical.
		if cand.Found >= 2 && sprite.MotionPresence(extracted.Frames) < 0.01 {
			cand.Warnings = append(cand.Warnings,
				"There is almost no movement between frames. Consider enriching the motion description so the action is clearer and regenerating.")
		}
		errCount := len(insp.Errors)

		// succeed immediately if the frame count is exact and there are no serious quality issues
		if cand.Found == expected && insp.Ok() {
			saveGalleryFrames(args.State.Name, extracted.Frames)
			return cand, nil
		}
		// update the best candidate: frame count first, then fewer errors on a tie
		score := cand.Found*100 - errCount*10
		if score > bestScore {
			best, bestScore, bestImgs = cand, score, extracted.Frames
		}
		lastErr = nil

		// correction feedback for the next attempt (keep the user's feedback + add measurement-based correction instructions)
		var fixes []string
		if cand.Found != expected {
			var layout string
			if rows, cols := sprite.GridForFrames(expected); rows > 1 {
				layout = fmt.Sprintf("a clean %d-column by %d-row grid, read left to right then top to bottom, with one clearly separated pose per cell and a wide magenta gutter between every row AND column so nothing touches or bridges", cols, rows)
			} else {
				layout = fmt.Sprintf("one horizontal row of %d equally sized poses, one clearly separated pose per column, each ringed by a clean magenta gap so none touch or overlap", expected)
			}
			fixes = append(fixes, fmt.Sprintf(
				"IMPORTANT CORRECTION: the last attempt read as %d poses but EXACTLY %d are required. Redraw as %s. Do not draw any frame, border, or film strip.",
				cand.Found, expected, layout))
		}
		if len(insp.RetryHints) > 0 {
			fixes = append(fixes, "QUALITY CORRECTIONS detected by automated inspection (fix all of these):")
			fixes = append(fixes, insp.RetryHints...)
		}
		auto := strings.Join(fixes, "\n")
		if args.Feedback != "" {
			feedback = args.Feedback + "\n" + auto
		} else {
			feedback = auto
		}

		// update the message before the final attempt: indicate this is a quality-correction retry
		if cand.Found == expected && errCount > 0 && attempt < maxAttempts {
			a.emit("progress", map[string]any{"phase": "generate", "state": args.State.Name,
				"message": fmt.Sprintf("Regenerating to improve quality... (%d/%d)", attempt+1, maxAttempts)})
		}
	}

	if best.Found == 0 {
		if lastErr != nil {
			return res, lastErr
		}
		return res, errors.New("could not extract sprite frames. Try writing a more specific character description")
	}
	if best.Found != expected {
		best.Warnings = append(best.Warnings,
			fmt.Sprintf("The frame count still differs after automatic retries (requested %d → extracted %d). Showing the closest result.", expected, best.Found))
	} else {
		best.Warnings = append(best.Warnings,
			"Some quality issues remain even after automatic retries. Review the frames and, if needed, regenerate with feedback.")
	}
	saveGalleryFrames(args.State.Name, bestImgs)
	return best, nil
}

// ---------- 8-direction set ----------

// ListDirections returns the list of 8-direction metadata (in 3x3 grid order).
func (a *App) ListDirections() []sprite.DirectionInfo {
	return sprite.ListDirections()
}

// ListPresets returns the catalog of 100 situation keywords (for the preset selection UI).
func (a *App) ListPresets() []sprite.PresetInfo {
	return sprite.ListPresets()
}

// MirrorFrames flips frames horizontally (for generating mirrored directions like east→west).
func (a *App) MirrorFrames(frames []string) ([]string, error) {
	out := make([]string, 0, len(frames))
	for i, fu := range frames {
		raw, err := decodeDataURL(fu)
		if err != nil {
			return nil, fmt.Errorf("failed to decode frame %d: %w", i+1, err)
		}
		img, err := decodeImage(raw)
		if err != nil {
			return nil, err
		}
		u, err := pngDataURL(sprite.MirrorNRGBA(sprite.ToNRGBA(img)))
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// ---------- Export ----------

// ExportState is the per-state data for export.
type ExportState struct {
	Name   string   `json:"name"`
	FPS    int      `json:"fps"`
	Loop   bool     `json:"loop"`
	Frames []string `json:"frames"` // selected/ordered dataURLs
}

// ExportArgs is a project export request.
type ExportArgs struct {
	Character string        `json:"character"`
	CellSize  int           `json:"cellSize"`
	States    []ExportState `json:"states"`
}

// ExportProject prompts for a directory and saves the sprite sheet, manifest, GIFs, and frames.
func (a *App) ExportProject(args ExportArgs) (string, error) {
	if len(args.States) == 0 {
		return "", errors.New("there are no animations to export")
	}
	cellSize := args.CellSize
	if cellSize <= 0 {
		cellSize = 256
	}

	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select export folder",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // canceled
	}

	charName := strings.TrimSpace(args.Character)
	if charName == "" {
		charName = "character"
	}
	safeName := sanitizeName(charName)
	outDir := filepath.Join(dir, safeName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	defer a.emit("progress", map[string]any{"phase": "idle", "message": ""})
	a.emit("progress", map[string]any{"phase": "export", "message": "Exporting sprite sheet and GIFs..."})

	// decode state frames
	var stateFrames []sprite.StateFrames
	for _, st := range args.States {
		if len(st.Frames) == 0 {
			continue
		}
		sf := sprite.StateFrames{
			Spec: sprite.StateSpec{Name: st.Name, Frames: len(st.Frames), FPS: st.FPS, Loop: st.Loop},
		}
		for _, fu := range st.Frames {
			raw, err := decodeDataURL(fu)
			if err != nil {
				return "", fmt.Errorf("failed to decode %s frame: %w", st.Name, err)
			}
			img, err := decodeImage(raw)
			if err != nil {
				return "", err
			}
			sf.Frames = append(sf.Frames, sprite.ToNRGBA(img))
		}
		stateFrames = append(stateFrames, sf)
	}
	if len(stateFrames) == 0 {
		return "", errors.New("there are no frames to export")
	}

	// filename prefix: prepend the character name to every file so outputs from different characters stay distinct when mixed.
	prefix := safeName + "-"

	// 1) sprite sheet + manifest
	sheet, manifest := sprite.ComposeAtlas(safeName, stateFrames, cellSize, cellSize)
	if err := writePNG(filepath.Join(outDir, prefix+"sprite-sheet.png"), sheet); err != nil {
		return "", err
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, prefix+"manifest.json"), manifestJSON, 0o644); err != nil {
		return "", err
	}
	// Aseprite-compatible JSON (for Phaser/Unity/Godot importers)
	aseJSON, err := sprite.BuildAsepriteJSON(manifest)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, prefix+"sprite-sheet.json"), aseJSON, 0o644); err != nil {
		return "", err
	}

	// 2) per-state GIF + individual frame PNGs
	for _, sf := range stateFrames {
		stateDir := filepath.Join(outDir, "frames", sanitizeName(sf.Spec.Name))
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return "", err
		}
		for i, frame := range sf.Frames {
			if err := writePNG(filepath.Join(stateDir, fmt.Sprintf("%sframe-%02d.png", prefix, i)), frame); err != nil {
				return "", err
			}
		}
		stateName := sanitizeName(sf.Spec.Name)
		gifBytes, err := sprite.EncodeGIF(sf.Frames, sf.Spec.FPS, sf.Spec.Loop)
		if err != nil {
			return "", fmt.Errorf("failed to encode %s GIF: %w", sf.Spec.Name, err)
		}
		gifDir := filepath.Join(outDir, "gif")
		if err := os.MkdirAll(gifDir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(gifDir, prefix+stateName+".gif"), gifBytes, 0o644); err != nil {
			return "", err
		}
		// APNG: full alpha support (compensates for GIF's 1-bit transparency limitation)
		apngBytes, err := sprite.EncodeAPNG(sf.Frames, sf.Spec.FPS, sf.Spec.Loop)
		if err != nil {
			return "", fmt.Errorf("failed to encode %s APNG: %w", sf.Spec.Name, err)
		}
		apngDir := filepath.Join(outDir, "apng")
		if err := os.MkdirAll(apngDir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(apngDir, prefix+stateName+".png"), apngBytes, 0o644); err != nil {
			return "", err
		}
	}

	return outDir, nil
}

// RawStripItem is one state's pre-slice transparent strip, captured straight
// from the stage. It is the exact input to sprite.ExtractFrames, so exporting
// these lets us iterate on the slicer/normalizer offline with zero generation cost.
type RawStripItem struct {
	Name     string `json:"name"`     // state name (export prefix)
	RawStrip string `json:"rawStrip"` // dataURL of the cleaned, pre-slice strip
	Expected int    `json:"expected"` // intended pose/frame count for this strip
}

// ExportRawStrips prompts for a directory and writes each state's pre-slice strip
// as <name>-raw.png, plus a strips.json mapping file → expected pose count. These
// are testdata for offline slicing tests; no API/token cost is incurred.
func (a *App) ExportRawStrips(items []RawStripItem) (string, error) {
	type manifestEntry struct {
		File     string `json:"file"`
		Name     string `json:"name"`
		Expected int    `json:"expected"`
	}
	hasAny := false
	for _, it := range items {
		if strings.TrimSpace(it.RawStrip) != "" {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return "", errors.New("there are no raw strips to export — generate some animations first")
	}
	var manifest []manifestEntry

	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select folder for raw strips (slicing testdata)",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // canceled
	}
	outDir := filepath.Join(dir, "raw-strips")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	for _, it := range items {
		if strings.TrimSpace(it.RawStrip) == "" {
			continue
		}
		raw, err := decodeDataURL(it.RawStrip)
		if err != nil {
			return "", fmt.Errorf("failed to decode %s strip: %w", it.Name, err)
		}
		img, err := decodeImage(raw)
		if err != nil {
			return "", err
		}
		file := sanitizeName(it.Name) + "-raw.png"
		if err := writePNG(filepath.Join(outDir, file), sprite.ToNRGBA(img)); err != nil {
			return "", err
		}
		manifest = append(manifest, manifestEntry{File: file, Name: it.Name, Expected: it.Expected})
	}
	if len(manifest) == 0 {
		return "", errors.New("there are no raw strips to export — generate some animations first")
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, "strips.json"), manifestJSON, 0o644); err != nil {
		return "", err
	}
	return outDir, nil
}

// RevealInFinder opens the exported folder in the file explorer.
func (a *App) RevealInFinder(path string) {
	runtime.BrowserOpenURL(a.ctx, "file://"+path)
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

var unsafeNameRe = regexp.MustCompile(`[^a-zA-Z0-9가-힣_-]+`)

func sanitizeName(name string) string {
	s := unsafeNameRe.ReplaceAllString(strings.TrimSpace(name), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "character"
	}
	return s
}
