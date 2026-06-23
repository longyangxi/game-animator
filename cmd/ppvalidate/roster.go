package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"time"

	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// rosterChar is a validation character (name + description). The name is used as the sample/<name> folder.
type rosterChar struct{ Name, Desc string }

// rosterChars is a collection of named characters for character-diversity validation.
// They use palettes that avoid the key colors (magenta/pink/purple) to prevent matting conflicts.
var rosterChars = []rosterChar{
	{"knight", "a stout knight in dented steel armor with a blue surcoat and a round shield"},
	{"archer", "a nimble green-hooded archer with a leather quiver and a longbow"},
	{"fire-mage", "a small fire mage in orange robes holding a gnarled wooden staff"},
	{"slime", "a round blue slime creature with big shiny eyes"},
	{"robot", "a sleek silver combat robot with glowing cyan eye sensors"},
	{"ninja", "a hooded ninja in dark navy with a steel katana on the back"},
	{"viking", "a bearded viking warrior with a horned helmet and a battle axe"},
	{"fairy", "a tiny winged fairy in a leaf-green dress with translucent wings"},
	{"ranger", "a rugged desert ranger in tan cloak with a brass revolver"},
	{"golem", "a chunky stone golem with mossy cracks and glowing core"},
	{"skeleton", "a grinning skeleton swordsman in tattered brown rags"},
	{"cat-warrior", "an orange tabby cat warrior standing upright in light armor"},
	{"diver", "a deep-sea diver in a heavy brass helmet and navy suit"},
	{"chef", "a cheerful chef in a white apron wielding a frying pan"},
	{"frost-knight", "a frost knight in pale blue plate armor with an icy longsword"},
	{"inventor", "a steampunk inventor in goggles and a brown leather coat"},
	{"explorer", "a jungle explorer in khaki with a wide-brim hat and whip"},
	{"paladin", "a golden-armored paladin with a tall tower shield"},
	{"courier-fox", "a swift courier fox in a teal running outfit"},
	{"ogre", "a hulking green ogre with a wooden club and a loincloth"},
	{"owl-drone", "a mechanical owl drone with copper plating and lantern eyes"},
	{"mummy", "a sand-colored mummy wrapped in old bandages"},
	{"farmer", "a young farmer in blue overalls holding a pitchfork"},
	{"vampire-hunter", "a crimson-cloaked vampire hunter with a silver crossbow"},
}

// rosterStates is the state rotation order used to spread out situation/direction diversity.
var rosterStates = []string{
	"idle", "walk", "attack", "jump", "run", "cast", "hurt", "wave",
	"slash", "death", "dash", "cheer", "block", "kick", "roll", "dance",
	"crouch", "climb", "shoot", "victory",
}

// rosterFacings is the rotation pattern that assigns a facing direction to some states.
var rosterFacings = []string{"", "", "east", "south-east", "north"}

// rosterStyleKey: unifies all base characters under the pixel-art style.
const rosterStyleKey = "pixel"

// genCharacter generates and scores one character's base + statesPer states and returns the results.
// It assigns states/directions deterministically via startIdx, so it is safe for concurrent execution.
func genCharacter(ctx context.Context, s gen.Provider, rc rosterChar, statesPer, startIdx int, outDir string) []stripResult {
	var out []stripResult
	style := sprite.ResolveStyle(rosterStyleKey, "")
	charDir := filepath.Join(outDir, rc.Name)

	t0 := time.Now()
	fmt.Printf("[%s] generating base (pixel)...\n", rc.Name)
	craw, err := s.GenerateImage(ctx, sprite.BuildCharacterPrompt(rc.Desc, style, ""), nil, "1:1")
	if err != nil {
		fmt.Printf("[%s] base failed: %v\n", rc.Name, err)
		return out
	}
	cimg, err := decode(craw)
	if err != nil {
		fmt.Printf("[%s] decode failed: %v\n", rc.Name, err)
		return out
	}
	baseClean := sprite.RemoveBackground(cimg)
	if pal := sprite.PaletteSizeForStyle(rosterStyleKey); pal > 0 {
		single := []*image.NRGBA{baseClean}
		sprite.PixelPostProcess(single, pal)
		baseClean = single[0]
	}
	_ = os.MkdirAll(charDir, 0o755)
	savePNG(filepath.Join(charDir, "base.png"), baseClean)
	baseBytes := pngBytes(baseClean)
	fmt.Printf("[%s] base done (%.0fs)\n", rc.Name, time.Since(t0).Seconds())

	for si := 0; si < statesPer; si++ {
		idx := startIdx + si
		name := rosterStates[idx%len(rosterStates)]
		facing := rosterFacings[idx%len(rosterFacings)]
		pre, ok := sprite.PresetByName(name)
		if !ok {
			continue
		}
		spec := sprite.StateSpec{Name: name, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action, Facing: facing}
		label := name
		if facing != "" {
			label = name + "-" + facing
			spec.Name = label
		}
		var baseN *image.NRGBA
		if !sprite.IsBackFacing(facing) {
			baseN = baseClean
		}
		ts := time.Now()
		res, err := genStrip(ctx, s, rc.Desc, rosterStyleKey, style, spec, [][]byte{baseBytes}, baseN)
		if err != nil {
			fmt.Printf("[%s/%s] error: %v\n", rc.Name, label, err)
			out = append(out, stripResult{Name: label, Expected: pre.Frames, rel: rc.Name, Errors: []string{err.Error()}})
			continue
		}
		res.Name = label
		res.rel = rc.Name
		saveFrames(charDir, res)
		fmt.Printf("[%s/%s] score%d %d/%d %s (%.0fs)\n", rc.Name, label, res.Score, res.Found, res.Expected, status(res), time.Since(ts).Seconds())
		out = append(out, res)
	}
	return out
}

// runRoster generates `chars` characters in parallel, grouped into batches of batchSize.
// Each character processes its base+states sequentially internally, but characters within a batch run concurrently.
func runRoster(ctx context.Context, s gen.Provider, chars, statesPer, batchSize int, outDir string) []stripResult {
	if batchSize < 1 {
		batchSize = 1
	}
	var (
		results []stripResult
		mu      sync.Mutex
	)
	for start := 0; start < chars; start += batchSize {
		end := start + batchSize
		if end > chars {
			end = chars
		}
		fmt.Printf("\n=== batch %d~%d / %d (%d concurrent) ===\n", start+1, end, chars, end-start)
		var wg sync.WaitGroup
		for ci := start; ci < end; ci++ {
			wg.Add(1)
			go func(ci int) {
				defer wg.Done()
				rc := rosterChars[ci%len(rosterChars)]
				r := genCharacter(ctx, s, rc, statesPer, ci*statesPer, outDir)
				mu.Lock()
				results = append(results, r...)
				mu.Unlock()
			}(ci)
		}
		wg.Wait()
	}
	return results
}
