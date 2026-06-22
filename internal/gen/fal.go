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

// Fal is a client for the fal.ai synchronous run API.
type Fal struct {
	APIKey string
	Model  string // e.g. fal-ai/nano-banana-pro (automatically uses /edit when reference images are present)
	HTTP   *http.Client
}

// NewFal creates a new fal.ai client.
func NewFal(apiKey, model string) *Fal {
	if model == "" {
		model = DefaultModelFor(ProviderFal)
	}
	return &Fal{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 300 * time.Second},
	}
}

type falRequest struct {
	Prompt       string   `json:"prompt"`
	ImageURLs    []string `json:"image_urls,omitempty"`
	NumImages    int      `json:"num_images"`
	OutputFormat string   `json:"output_format"`
	AspectRatio  string   `json:"aspect_ratio,omitempty"`
	SyncMode     bool     `json:"sync_mode"`
}

type falResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Detail any `json:"detail"`
}

// endpoint selects the edit endpoint depending on whether reference images are present.
func (c *Fal) endpoint(hasRefs bool) string {
	model := strings.TrimSuffix(strings.TrimSpace(c.Model), "/")
	if hasRefs && !strings.HasSuffix(model, "/edit") {
		model += "/edit"
	}
	return "https://fal.run/" + model
}

// GenerateImage generates an image with fal.ai.
func (c *Fal) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("fal.ai API key is not set. Please enter it in the settings")
	}

	reqData := falRequest{
		// Also pass a prompt hint in case the aspect ratio parameter is ignored.
		Prompt:       prompt + "\n\n" + aspectHint(aspectRatio),
		NumImages:    1,
		OutputFormat: "png",
		AspectRatio:  aspectRatio,
		SyncMode:     false,
	}
	for _, img := range refImages {
		reqData.ImageURLs = append(reqData.ImageURLs,
			"data:image/png;base64,"+base64.StdEncoding.EncodeToString(img))
	}

	url := c.endpoint(len(refImages) > 0)

	var lastErr error
	backoff := 2 * time.Second
	withAspect := true
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		send := reqData
		if !withAspect {
			send.AspectRatio = ""
		}
		body, err := json.Marshal(send)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request: %w", err)
		}

		img, status, err := c.doRequest(ctx, url, body)
		if err == nil {
			return img, nil
		}
		lastErr = err

		// On a schema rejection (422), retry once without aspect_ratio.
		if status == 422 && withAspect {
			withAspect = false
			continue
		}
		if status != 429 && status < 500 && status != 0 {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Fal) doRequest(ctx context.Context, url string, body []byte) (img []byte, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var parsed falResponse
		_ = json.Unmarshal(respBytes, &parsed)
		detail := ""
		if parsed.Detail != nil {
			if d, jerr := json.Marshal(parsed.Detail); jerr == nil {
				detail = ": " + string(d)
			}
		}
		return nil, resp.StatusCode, fmt.Errorf("fal.ai error (HTTP %d)%s", resp.StatusCode, detail)
	}

	var parsed falResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(parsed.Images) == 0 || parsed.Images[0].URL == "" {
		return nil, 500, errors.New("the response contains no image")
	}
	data, err := decodeDataOrDownload(c.HTTP, parsed.Images[0].URL)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// ValidateKey checks the fal key format (fal has no lightweight validation endpoint).
func (c *Fal) ValidateKey(_ context.Context) error {
	key := strings.TrimSpace(c.APIKey)
	if len(key) < 10 {
		return errors.New("the API key is too short")
	}
	return nil
}
