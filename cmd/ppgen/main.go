// Command ppgen is a headless CLI that generates a character plus animation
// states without a GUI and exports a game-engine-ready bundle to disk
// (sprite sheet, manifest.json, Aseprite JSON, per-state GIF/APNG, and
// individual frame PNGs).
//
// It reproduces the GenerateState + ExportProject logic of the installable
// Wails app, but exports directly to the -out directory without any file
// dialog and prints a result summary as JSON to stdout, making it easy to
// call from skills and scripts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"

	"perfectpixel/internal/config"
	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// options groups the run options assembled from CLI flags.
type options struct {
	desc     string
	style    string
	states   string
	percat   int
	all      bool
	dirset   string
	out      string
	provider string
	key      string
	model    string
	attempts int
	timeout  time.Duration
	jsonOut  bool
	quiet    bool
	baseOnly bool
}

func main() {
	var (
		opt  options
		dump = flag.Bool("dump", false, "print the preset+direction catalog as JSON and exit")
	)
	flag.StringVar(&opt.desc, "desc", "a small knight with silver armor and a blue plume on the helmet", "character description")
	flag.StringVar(&opt.style, "style", "pixel", "style key (pixel | chibi | cartoon | retro16)")
	flag.StringVar(&opt.states, "states", "idle,walk", "comma-separated list of state names to generate")
	flag.IntVar(&opt.percat, "percat", 0, "if greater than 0, auto-select N presets per category (ignores states)")
	flag.BoolVar(&opt.all, "all", false, "generate all presets (ignores states/percat)")
	flag.StringVar(&opt.dirset, "dirset", "", "state name to additionally generate as an 8-direction set (optional)")
	flag.StringVar(&opt.out, "out", "./perfectpixel-out", "output directory")
	flag.StringVar(&opt.provider, "provider", "", "force a specific provider (gemini|openrouter|fal|byteplus)")
	flag.StringVar(&opt.key, "key", "", "force a specific API key (takes precedence over config/env vars)")
	flag.StringVar(&opt.model, "model", "", "force a specific model")
	flag.IntVar(&opt.attempts, "attempts", 3, "max retries per state for quality-correction regeneration")
	flag.DurationVar(&opt.timeout, "timeout", 30*time.Minute, "overall timeout")
	flag.BoolVar(&opt.jsonOut, "json", false, "print only the result summary JSON to stdout instead of human-readable logs")
	flag.BoolVar(&opt.quiet, "quiet", false, "suppress progress logs (pairs well with -json)")
	flag.BoolVar(&opt.baseOnly, "baseonly", false, "generate only the base character (base.png) and skip states/bundle")
	flag.Parse()

	if *dump {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"presets":    sprite.ListPresets(),
			"directions": sprite.ListDirections(),
			"styles":     []string{"pixel", "chibi", "cartoon", "retro16"},
			"providers":  gen.SupportedProviders,
		})
		return
	}

	if err := runGen(opt); err != nil {
		fmt.Fprintf(os.Stderr, "ppgen failed: %v\n", err)
		os.Exit(1)
	}
}

// resolveProvider reads config/env vars, applies CLI overrides, and builds the provider.
func resolveProvider(opt options) (gen.Provider, string, string, error) {
	s := config.Load()
	provider := s.Provider
	if opt.provider != "" {
		provider = opt.provider
	}
	cfg := s.Cfg(provider)
	key := cfg.APIKey
	if opt.key != "" {
		key = opt.key
	}
	model := cfg.Model
	if opt.model != "" {
		model = opt.model
	}
	if key == "" {
		return nil, "", "", fmt.Errorf("no API key for provider %q (use config.json, .env, an environment variable, or -key)", provider)
	}
	p, err := gen.New(provider, key, model)
	if err != nil {
		return nil, "", "", err
	}
	if model == "" {
		model = gen.DefaultModelFor(provider)
	}
	return p, provider, model, nil
}

// selectStates picks the presets to generate, in -all / -percat / -states priority order.
func selectStates(opt options) ([]sprite.PresetInfo, error) {
	byName := map[string]sprite.PresetInfo{}
	for _, p := range sprite.Presets {
		byName[p.Name] = p
	}
	if opt.all {
		return append([]sprite.PresetInfo(nil), sprite.Presets...), nil
	}
	if opt.percat > 0 {
		count := map[string]int{}
		var out []sprite.PresetInfo
		for _, p := range sprite.Presets {
			if count[p.Category] < opt.percat {
				out = append(out, p)
				count[p.Category]++
			}
		}
		return out, nil
	}
	var out []sprite.PresetInfo
	var missing []string
	for _, n := range strings.Split(opt.states, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if p, ok := byName[n]; ok {
			out = append(out, p)
		} else {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("unknown state name: %s (see -dump for the list)", strings.Join(missing, ", "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no states to generate (specify -states, -percat, or -all)")
	}
	return out, nil
}
