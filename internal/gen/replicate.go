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

// Replicate is a client for the Replicate prediction API. It targets image models that fit this
// project's reference-guided strip workflow — primarily Google's Gemini-family image models
// (image_input + aspect_ratio), with a cheap SDXL-style fallback for quick pipeline checks.
type Replicate struct {
	APIKey string
	Model  string // e.g. google/nano-banana-2
	HTTP   *http.Client
}

// NewReplicate creates a new Replicate client.
func NewReplicate(apiKey, model string) *Replicate {
	if model == "" {
		model = DefaultModelFor(ProviderReplicate)
	}
	return &Replicate{
		APIKey: apiKey,
		Model:  strings.TrimSpace(model),
		HTTP:   &http.Client{Timeout: 300 * time.Second},
	}
}

const replicateAPIBase = "https://api.replicate.com/v1"

type replPrediction struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Output json.RawMessage `json:"output"`
	Error  string          `json:"error"`
	URLs   struct {
		Get string `json:"get"`
	} `json:"urls"`
	Detail string `json:"detail"`
	Title  string `json:"title"`
}

// GenerateImage runs a Replicate prediction and returns the generated image bytes. Background
// removal is handled later by the sprite pipeline, so this returns the raw image (png or jpg).
func (c *Replicate) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("Replicate API token is not set. Please enter it in the settings")
	}
	// Only Gemini-family image models are offered for Replicate (see provider.go): they accept a
	// fixed aspect set and reference images, which is what this strip/grid workflow needs.
	input := map[string]any{
		// Also pass a prompt hint in case the aspect parameter is ignored/snapped.
		"prompt":        prompt + "\n\n" + aspectHint(aspectRatio),
		"output_format": "png",
	}
	if ar := geminiSnapAspect(aspectRatio); ar != "" {
		input["aspect_ratio"] = ar
	}
	// Reference images drive character identity across the strip.
	if len(refImages) > 0 {
		uris := make([]string, 0, len(refImages))
		for _, img := range refImages {
			uris = append(uris, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(img))
		}
		input["image_input"] = uris
	}

	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	pred, err := c.createPrediction(ctx, replicateAPIBase+"/models/"+c.Model+"/predictions", body)
	if err != nil {
		return nil, err
	}
	if pred.Status != "succeeded" {
		pred, err = c.poll(ctx, pred)
		if err != nil {
			return nil, err
		}
	}
	if pred.Status != "succeeded" {
		if pred.Error != "" {
			return nil, fmt.Errorf("Replicate prediction %s: %s", pred.Status, pred.Error)
		}
		return nil, fmt.Errorf("Replicate prediction %s", pred.Status)
	}
	url := firstReplOutput(pred.Output)
	if url == "" {
		return nil, errors.New("no output image from Replicate")
	}
	return decodeDataOrDownload(c.HTTP, url)
}

// ValidateKey checks the token against the account endpoint.
func (c *Replicate) ValidateKey(ctx context.Context) error {
	if c.APIKey == "" {
		return errors.New("Replicate API token is not set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, replicateAPIBase+"/account", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("invalid Replicate API token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Replicate validation failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// createPrediction POSTs a prediction with Prefer:wait so it returns synchronously when fast.
// On a 429 throttle (Replicate caps accounts with < $5 credit to 6 req/min, burst 1) or a 5xx,
// it waits and retries instead of failing — a throttle becomes a short delay, not a hard error.
func (c *Replicate) createPrediction(ctx context.Context, endpoint string, body []byte) (*replPrediction, error) {
	backoff := 8 * time.Second
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "wait")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("network error: %w", err)
			continue // transient network error → retry
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		var pred replPrediction
		_ = json.Unmarshal(data, &pred)
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			return &pred, nil
		}
		msg := firstNonEmptyStr(pred.Detail, pred.Title, string(data))
		lastErr = fmt.Errorf("Replicate error (%d): %s", resp.StatusCode, msg)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			continue // throttle / server error → wait and retry
		}
		return nil, lastErr // other 4xx (bad token, bad input) → fail fast
	}
	return nil, lastErr
}

// poll waits for an async prediction to reach a terminal state.
func (c *Replicate) poll(ctx context.Context, pred *replPrediction) (*replPrediction, error) {
	getURL := pred.URLs.Get
	if getURL == "" {
		if pred.ID == "" {
			return nil, errors.New("no polling URL from Replicate")
		}
		getURL = replicateAPIBase + "/predictions/" + pred.ID
	}
	for i := 0; i < 90; i++ { // ~3 min at 2s intervals
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		var p replPrediction
		if json.Unmarshal(data, &p) != nil {
			continue
		}
		switch p.Status {
		case "succeeded", "failed", "canceled":
			return &p, nil
		}
	}
	return nil, errors.New("Replicate prediction timed out")
}

// firstReplOutput extracts the first image URL/data-URI from an output that may be a string or array.
func firstReplOutput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
