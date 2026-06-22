# PerfectPixel providers / models / configuration

`ppgen` supports four image-generation backends. The active provider and key are
resolved in the following order.

1. Installed app settings file `~/.config/perfectpixel/config.json` (highest priority if present)
2. `.env` / `.env.local` next to the working directory or executable
3. OS environment variables
4. CLI flags `-provider` / `-key` / `-model` (override all of the above; forced)

If no provider is specified, the first provider with a configured key is auto-selected;
the default is `gemini`.

## Environment variables · models per provider

| Provider | `-provider` value | API key environment variable | Default model | Alternative models |
|---|---|---|---|---|
| Gemini (Google AI Studio) | `gemini` | `GEMINI_API_KEY` / `GOOGLE_API_KEY` | `gemini-3-pro-image` | `gemini-3-pro-image-preview`, `gemini-2.5-flash-image` |
| OpenRouter | `openrouter` | `OPENROUTER_API_KEY` | `google/gemini-3-pro-image-preview` | `google/gemini-2.5-flash-image` |
| fal.ai | `fal` | `FAL_KEY` / `FAL_API_KEY` | `fal-ai/nano-banana-pro` | `fal-ai/nano-banana`, `fal-ai/flux-pro/v1.1-ultra` |
| BytePlus (ARK) | `byteplus` | `BYTEPLUS_API_KEY` / `ARK_API_KEY` | `seedream-4-0-250828` | `seedream-3-0-t2i-250415` |

## .env example

```dotenv
# Fill in only the key for the provider you want to use.
GEMINI_API_KEY=
OPENROUTER_API_KEY=
FAL_KEY=
BYTEPLUS_API_KEY=
```

## Style keys (`-style`)

- `pixel` — true dot-style pixel art (shared-palette quantization + grid snapping), default
- `chibi` — cute 2–3 head-tall proportions
- `cartoon` — soft cartoon style
- `retro16` — 16-bit console-style limited palette

## Quality / cost tips

- The `gemini-3-pro-image` (Nano Banana Pro) family gives the best character
  consistency / motion quality.
- Each state is regenerated up to `-attempts` times (default 3), so before a large batch
  you can save cost by doing a trial run with a single `-states idle` to check
  style/quality.
- `-all` (100+ states) makes a very large number of calls. Warn the user about the scale
  before running.
