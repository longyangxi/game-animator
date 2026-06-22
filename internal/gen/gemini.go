// Package gen provides clients for AI image generation providers.
package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultModel is the default image generation model (Nano Banana Pro / Gemini 3 Pro Image).
// Its greatly improved reference-image identity preservation and adherence to complex
// layout instructions make it the best choice for frame-to-frame stability in sprite strips.
const DefaultModel = "gemini-3-pro-image"

// modelFallbacks is the fallback chain for keys/regions where the default model is not yet available.
// It is tried in order only on a 404 (model not found).
var modelFallbacks = []string{
	"gemini-3-pro-image-preview",
	"gemini-3.1-flash-image",
	"gemini-2.5-flash-image",
}

var errModelNotFound = errors.New("the requested model could not be found")

const apiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// Client is a client for the Gemini image generation API.
type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client

	endpoint string // test override (uses apiEndpoint when empty)
}

// NewClient creates a new Gemini client.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 180 * time.Second},
	}
}

type genRequest struct {
	Contents         []genContent `json:"contents"`
	GenerationConfig *genConfig   `json:"generationConfig,omitempty"`
}

type genContent struct {
	Parts []genPart `json:"parts"`
}

type genPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type genConfig struct {
	ResponseModalities []string     `json:"responseModalities,omitempty"`
	ImageConfig        *imageConfig `json:"imageConfig,omitempty"`
}

type imageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

type genResponse struct {
	Candidates []struct {
		Content struct {
			Parts []genPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// geminiSnapAspect maps an arbitrary "W:H" ratio onto the fixed set Gemini accepts.
// AspectForFrames may request wide custom ratios (e.g. 32:9) that only the computed-size
// providers support; for Gemini we snap to its widest landscape values so the request stays valid.
func geminiSnapAspect(aspectRatio string) string {
	w, h := parseAspect(aspectRatio)
	if w <= 0 || h <= 0 {
		return aspectRatio // e.g. "1:1" passes through
	}
	r := float64(w) / float64(h)
	switch {
	case r >= 21.0/9.0:
		return "21:9" // anything 21:9 or wider → Gemini's widest landscape
	case r >= 16.0/9.0:
		return "16:9"
	default:
		return aspectRatio
	}
}

// GenerateImage generates an image from a prompt and reference images (PNG bytes).
// aspectRatio supports values like "1:1", "16:9", "21:9"; it is omitted when empty.
func (c *Client) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("API key is not set. Please enter your Gemini API key in the settings")
	}

	parts := make([]genPart, 0, len(refImages)+1)
	for _, img := range refImages {
		parts = append(parts, genPart{
			InlineData: &inlineData{
				MimeType: "image/png",
				Data:     base64.StdEncoding.EncodeToString(img),
			},
		})
	}
	parts = append(parts, genPart{Text: prompt})

	// Per-model request body: Gemini 3 Pro supports 2K resolution on wide strips, which
	// increases the pixels per frame and greatly improves extraction quality.
	// Fallback models do not support imageSize, so the body is rebuilt depending on the model.
	buildBody := func(model string) ([]byte, error) {
		cfg := &genConfig{ResponseModalities: []string{"TEXT", "IMAGE"}}
		if aspectRatio != "" {
			ar := geminiSnapAspect(aspectRatio)
			ic := &imageConfig{AspectRatio: ar}
			if strings.HasPrefix(model, "gemini-3-pro") && ar != "1:1" {
				ic.ImageSize = "2K"
			}
			cfg.ImageConfig = ic
		}
		return json.Marshal(genRequest{
			Contents:         []genContent{{Parts: parts}},
			GenerationConfig: cfg,
		})
	}
	reqBody, err := buildBody(c.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	var lastErr error
	backoff := 2 * time.Second
	model := c.Model
	fallbacks := modelFallbacks
	tried := map[string]bool{model: true}
	for attempt := 0; attempt < 3; {
		img, retryable, err := c.doRequest(ctx, model, reqBody)
		if err == nil {
			return img, nil
		}
		lastErr = err

		// Model not available (404) -> switch to the fallback chain immediately (does not consume an attempt).
		if errors.Is(err, errModelNotFound) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			for len(fallbacks) > 0 && tried[fallbacks[0]] {
				fallbacks = fallbacks[1:]
			}
			if len(fallbacks) == 0 {
				return nil, err
			}
			model = fallbacks[0]
			fallbacks = fallbacks[1:]
			tried[model] = true
			if reqBody, err = buildBody(model); err != nil {
				return nil, fmt.Errorf("failed to serialize request: %w", err)
			}
			continue
		}
		if !retryable {
			return nil, err
		}
		attempt++
		if attempt >= 3 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, lastErr
}

func (c *Client) doRequest(ctx context.Context, model string, body []byte) (img []byte, retryable bool, err error) {
	ep := c.endpoint
	if ep == "" {
		ep = apiEndpoint
	}
	url := fmt.Sprintf(ep, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// http.Client.Timeout also comes wrapped as context.DeadlineExceeded. Retrying a request
		// that has already used up its full timeout would make the UI appear frozen up to 3x longer.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, false, fmt.Errorf("model %q: %w", model, errModelNotFound)
		}
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500
		var parsed genResponse
		if json.Unmarshal(respBytes, &parsed) == nil && parsed.Error != nil {
			return nil, retryable, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("Gemini API error (HTTP %d)", resp.StatusCode)
	}

	var parsed genResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, false, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(parsed.Candidates) == 0 {
		return nil, true, errors.New("the image generation result is empty")
	}
	for _, part := range parsed.Candidates[0].Content.Parts {
		if part.InlineData != nil && part.InlineData.Data != "" {
			data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				return nil, false, fmt.Errorf("failed to decode image: %w", err)
			}
			return data, false, nil
		}
	}
	reason := parsed.Candidates[0].FinishReason
	return nil, true, fmt.Errorf("the response contains no image (reason: %s)", reason)
}

// ValidateKey performs a lightweight check of the API key's validity.
func (c *Client) ValidateKey(ctx context.Context) error {
	url := "https://generativelanguage.googleapis.com/v1beta/models?pageSize=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-goog-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		return errors.New("the API key is invalid")
	}
	return fmt.Errorf("key validation failed (HTTP %d)", resp.StatusCode)
}
