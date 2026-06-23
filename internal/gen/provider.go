package gen

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Supported provider identifiers.
const (
	ProviderGemini     = "gemini"
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderFal        = "fal"
	ProviderBytePlus   = "byteplus"
	ProviderReplicate  = "replicate"
)

// SupportedProviders is the list of supported provider identifiers (in UI display order).
var SupportedProviders = []string{ProviderGemini, ProviderOpenAI, ProviderOpenRouter, ProviderFal, ProviderBytePlus, ProviderReplicate}

// modelCatalog is the list of selectable image models per provider (newest model first).
var modelCatalog = map[string][]string{
	ProviderGemini: {
		"gemini-3-pro-image", // Nano Banana Pro (newest)
		"gemini-3-pro-image-preview",
		"gemini-2.5-flash-image", // Nano Banana
	},
	ProviderOpenAI: {
		"gpt-image-2",
		"gpt-image-1.5",
		"gpt-image-1",
		"gpt-image-1-mini",
	},
	ProviderOpenRouter: {
		"google/gemini-3-pro-image-preview", // newest
		"google/gemini-2.5-flash-image",
		"google/gemini-2.5-flash-image-preview",
	},
	ProviderFal: {
		"fal-ai/nano-banana-pro", // newest
		"fal-ai/nano-banana",
		"fal-ai/flux-pro/v1.1-ultra",
		"fal-ai/flux/dev",
	},
	ProviderBytePlus: {
		"seedream-4-0-250828",     // Seedream 4.0 (newest)
		"seedream-3-0-t2i-250415", // Seedream 3.0
		"seededit-3-0-i2i-250628", // SeedEdit 3.0 (image editing)
	},
	ProviderReplicate: {
		"google/nano-banana-2", // Gemini image, best identity for the strip workflow (default)
		"google/nano-banana",   // Gemini 2.5 Flash image — cheaper, the "for testing" option
		// Note: SDXL-style models (e.g. prunaai/z-image-turbo) are intentionally NOT offered — they
		// cannot produce the magenta-keyed, cleanly-separated multi-pose sprite sheets this pipeline
		// needs (extraction finds 0 poses), so only Gemini-family image models are listed here.
	},
}

// ModelsFor returns the list of selectable models offered by a provider (newest model first).
func ModelsFor(provider string) []string {
	if list, ok := modelCatalog[provider]; ok {
		return append([]string(nil), list...)
	}
	return nil
}

// Provider is the common interface for image generation backends.
type Provider interface {
	// GenerateImage generates an image from a prompt and reference images (PNG).
	GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error)
	// ValidateKey checks whether the API key is valid.
	ValidateKey(ctx context.Context) error
}

// DefaultModelFor returns the default model for a given provider.
func DefaultModelFor(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return "gpt-image-2"
	case ProviderOpenRouter:
		return "google/gemini-3-pro-image-preview"
	case ProviderFal:
		return "fal-ai/nano-banana-pro"
	case ProviderBytePlus:
		return "seedream-4-0-250828"
	case ProviderReplicate:
		return "google/nano-banana-2"
	default:
		return DefaultModel // gemini-3-pro-image (Nano Banana Pro)
	}
}

// ProviderLabel is the display name used in the UI.
func ProviderLabel(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return "OpenAI"
	case ProviderOpenRouter:
		return "OpenRouter"
	case ProviderFal:
		return "fal.ai"
	case ProviderBytePlus:
		return "BytePlus"
	case ProviderReplicate:
		return "Replicate"
	default:
		return "Gemini"
	}
}

// New creates a provider implementation.
func New(provider, apiKey, model string) (Provider, error) {
	if model == "" {
		model = DefaultModelFor(provider)
	}
	switch provider {
	case ProviderGemini, "":
		return NewClient(apiKey, model), nil
	case ProviderOpenAI:
		return NewOpenAI(apiKey, model), nil
	case ProviderOpenRouter:
		return NewOpenRouter(apiKey, model), nil
	case ProviderFal:
		return NewFal(apiKey, model), nil
	case ProviderBytePlus:
		return NewBytePlus(apiKey, model), nil
	case ProviderReplicate:
		return NewReplicate(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// aspectHint is a supplementary prompt phrase for APIs that do not support an aspect ratio parameter.
func aspectHint(aspectRatio string) string {
	switch aspectRatio {
	case "", "1:1":
		return "Render on a square 1:1 canvas."
	case "16:9":
		return "Render on a wide 16:9 landscape canvas, much wider than tall."
	case "21:9":
		return "Render on an extra-wide 21:9 panorama canvas, with plenty of horizontal room for a long row of poses."
	default:
		return fmt.Sprintf("Render on a wide %s landscape canvas.", aspectRatio)
	}
}

// decodeDataOrDownload decodes a data: URL, or downloads an http(s) URL.
func decodeDataOrDownload(httpClient *http.Client, url string) ([]byte, error) {
	if strings.HasPrefix(url, "data:") {
		idx := strings.Index(url, "base64,")
		if idx < 0 {
			return nil, errors.New("could not parse the image data format")
		}
		return base64.StdEncoding.DecodeString(url[idx+7:])
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download result image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download result image (HTTP %d)", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}
