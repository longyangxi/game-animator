---
name: perfectpixel
description: AI animated sprite generation. From a single line of text, create a character plus motion animations (walk, run, attack, magic, and 100+ more) and an 8-direction sprite set, then export them as a game-engine-ready bundle (sprite sheet · manifest.json · Aseprite JSON · per-state GIF/APNG · individual frame PNGs). Use when the user wants to generate game sprites, character animations, sprite sheets, sprite atlases, or 8-direction sprite sets from a text description.
user-invocable: true
allowed-tools:
  - Bash
  - Read
  - Write
---

# PerfectPixel — AI animated sprite generation

This drives the same generation pipeline as the installable desktop app (prompt → AI
image generation → background removal → frame extraction → quality inspection →
corrective regeneration → pixel quantization) through a headless CLI (`ppgen`) to
produce sprite bundles ready to drop into a game.

Use this skill when the user asks to generate a character, motion animations, a sprite
sheet, or an 8-direction set.

Arguments passed: `$ARGUMENTS`

---

## 0. Prerequisites (first step of every run)

Check for / install the `ppgen` binary. Run the install script from the directory that
contains this `SKILL.md` (the skill root). The install order is: ① reuse an existing
binary → ② **download a prebuilt binary matching the OS/arch from GitHub Releases (no Go
required)** → ③ if the download fails, build from Go source (or clone the public
repository). On success it **prints the binary's absolute path as the last line**.

```bash
# Replace SKILL_DIR with the absolute path of the directory containing this SKILL.md.
PPGEN="$(bash "$SKILL_DIR/scripts/install.sh" 2>/tmp/ppgen-install.log | tail -1)"
echo "$PPGEN"   # use this path for all subsequent ppgen calls
```

If installation fails (empty output or not executable), read `/tmp/ppgen-install.log`
and tell the user the cause (e.g. blocked network + Go not installed).

## 1. Check the API key

`ppgen` looks for a key in this order: `config.json` (installed app settings) → `.env` /
`.env.local` next to the working directory or executable → OS environment variables →
the CLI `-key` flag.

Supported providers and environment variables:

| Provider | Environment variable | Default model |
|---|---|---|
| gemini (default) | `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) | gemini-3-pro-image |
| openrouter | `OPENROUTER_API_KEY` | google/gemini-3-pro-image-preview |
| fal | `FAL_KEY` (or `FAL_API_KEY`) | fal-ai/nano-banana-pro |
| byteplus | `BYTEPLUS_API_KEY` (or `ARK_API_KEY`) | seedream-4-0-250828 |

If no key is found anywhere, ask the user which provider and key to use, or have them
specify `-provider`/`-key` directly. **Never guess or make up a key.**

## 2. Interpret the request → map to flags

Extract the following from the user's natural-language request and map them to `ppgen`
flags.

- Character description → `-desc "..."` (English prompts produce the best quality. If the
  description is in Korean, translate the essentials into English to pass through, but
  keep the original text in what you show the user.)
- Style → one of `-style`: `pixel` (default), `chibi`, `cartoon`, `retro16`
- Motions to create → `-states "idle,walk,attack"` (comma-separated). Motion names are
  English preset keys.
  - Check the full list/categories with `"$PPGEN" -dump` (also summarized in
    `reference/presets.md`).
  - For vague requests like "just the basic set" → recommend `-percat 1` (one per
    category) or the four core motions `idle,walk,run,attack`.
  - "Everything" → `-all` (100+, large in time/cost → warn the user about the scale
    first).
- 8-direction set request → specify one motion, e.g. `-dirset walk` (5 directions
  AI-generated + 3 mirrored).
- Output folder → `-out ./output-dir` (default `./perfectpixel-out`).

## 3. Run

Always run with `-json` to get a machine-readable summary. Generation takes tens of
seconds to several minutes per state, so run it in the background with a generous
timeout and wait for the completion notification.

```bash
"$PPGEN" \
  -desc "a small knight with silver armor and a blue plume" \
  -style pixel \
  -states "idle,walk,run,attack" \
  -out ./knight-sprites \
  -json
```

If cost/speed is a concern, first do a trial run with a single state (`-states idle`) to
check quality, then run the full set after the user approves.

## 4. Interpret and report the results

Key fields of the stdout JSON (`exportSummary`):

- `ok`, `outDir`, `provider`, `model`, `style`, `animations` (number of states),
  `sheetWidth/Height`
- `files`: list of generated artifacts
- `results[]`: per-state `{ found/expected, score(0–100), identity, motion, status, errors }`

If any state has `status` of `frame-mismatch` (frame count mismatch) or a low `score`
(<50), notify the user and suggest regenerating (raise `-attempts`, make the description
more specific, change the style).

The output bundle (`outDir/`):

```
base.png                      Base character
sprite-sheet.png              Sprite sheet (rows = states, columns = frames)
manifest.json                 PerfectPixel runtime metadata (schema v2)
sprite-sheet.json             Aseprite-compatible JSON (import into Phaser/Unity/Godot)
frames/<state>/frame-NN.png   Individual frames
gif/<state>.gif               Per-state animation preview
apng/<state>.png              Full-alpha animation
```

For the user, briefly explain the output path, the state count/quality summary, and how
to import into a game engine (usually the `sprite-sheet.png` + `sprite-sheet.json` pair).

---

## References

- Provider/model/environment-variable details: `reference/providers.md`
- Motion preset catalog: `reference/presets.md` (or `"$PPGEN" -dump`)
- This skill shares the **same** Go pipeline as the installable app. Algorithm changes
  are updated alongside the app, and `install.sh` rebuilds from the latest source.
