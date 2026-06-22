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
	"strconv"
	"strings"
	"time"
)

// bytePlusEndpoint is the URL of the BytePlus ModelArk image generation (Seedream) API.
const bytePlusEndpoint = "https://ark.ap-southeast.bytepluses.com/api/v3/images/generations"

// BytePlus is a BytePlus ModelArk (Seedream) image generation client.
type BytePlus struct {
	APIKey string
	Model  string // e.g. seedream-4-0-250828
	HTTP   *http.Client

	endpoint string // test override (uses bytePlusEndpoint when empty)
}

// NewBytePlus creates a new BytePlus client.
func NewBytePlus(apiKey, model string) *BytePlus {
	if model == "" {
		model = DefaultModelFor(ProviderBytePlus)
	}
	return &BytePlus{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 300 * time.Second},
	}
}

type bpRequest struct {
	Model                     string   `json:"model"`
	Prompt                    string   `json:"prompt"`
	Image                     []string `json:"image,omitempty"` // reference/edit images (data URL)
	Size                      string   `json:"size,omitempty"`
	ResponseFormat            string   `json:"response_format"`
	Watermark                 bool     `json:"watermark"`
	SequentialImageGeneration string   `json:"sequential_image_generation,omitempty"`
}

type bpResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateImage generates an image with BytePlus Seedream.
func (c *BytePlus) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("BytePlus API key is not set. Please enter it in the settings")
	}

	reqData := bpRequest{
		Model: c.Model,
		// Also pass a prompt hint in case the aspect ratio parameter is ignored.
		Prompt:                    prompt + "\n\n" + aspectHint(aspectRatio),
		Size:                      bpSizeFor(aspectRatio),
		ResponseFormat:            "b64_json",
		Watermark:                 false,
		SequentialImageGeneration: "disabled",
	}
	for _, img := range refImages {
		reqData.Image = append(reqData.Image,
			"data:image/png;base64,"+base64.StdEncoding.EncodeToString(img))
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	var lastErr error
	backoff := 2 * time.Second
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		img, retryable, err := c.doRequest(ctx, body)
		if err == nil {
			return img, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

// PLACEHOLDER_DOREQUEST

func (c *BytePlus) doRequest(ctx context.Context, body []byte) (img []byte, retryable bool, err error) {
	ep := c.endpoint
	if ep == "" {
		ep = bytePlusEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("failed to read response: %w", err)
	}

	var parsed bpResponse
	_ = json.Unmarshal(respBytes, &parsed)

	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500
		if parsed.Error != nil && parsed.Error.Message != "" {
			return nil, retryable, fmt.Errorf("BytePlus error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("BytePlus error (HTTP %d)", resp.StatusCode)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, true, fmt.Errorf("BytePlus error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return nil, true, errors.New("the response contains no image")
	}
	d := parsed.Data[0]
	if d.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(d.B64JSON)
		if err != nil {
			return nil, false, fmt.Errorf("failed to decode image: %w", err)
		}
		return data, false, nil
	}
	if d.URL != "" {
		data, err := decodeDataOrDownload(c.HTTP, d.URL)
		if err != nil {
			return nil, false, err
		}
		return data, false, nil
	}
	return nil, true, errors.New("the response contains no image")
}

// ValidateKey checks the BytePlus key format (there is no lightweight validation endpoint).
func (c *BytePlus) ValidateKey(_ context.Context) error {
	key := strings.TrimSpace(c.APIKey)
	if len(key) < 10 {
		return errors.New("the API key is too short")
	}
	return nil
}

// bpSizeFor converts an aspect ratio into a pixel size (WxH) allowed by Seedream.
// Each side is clamped to the [1280, 4096] range.
func bpSizeFor(aspectRatio string) string {
	const (
		minSide = 1280
		maxSide = 4096
	)
	w, h := parseAspect(aspectRatio)
	if w <= 0 || h <= 0 {
		return "2048x2048"
	}
	// Fit the long side to maxSide, then compute the short side proportionally and clamp to range.
	var pw, ph int
	if w >= h {
		pw = maxSide
		ph = int(float64(maxSide) * float64(h) / float64(w))
	} else {
		ph = maxSide
		pw = int(float64(maxSide) * float64(w) / float64(h))
	}
	clamp := func(v int) int {
		if v < minSide {
			return minSide
		}
		if v > maxSide {
			return maxSide
		}
		return v
	}
	return strconv.Itoa(clamp(pw)) + "x" + strconv.Itoa(clamp(ph))
}

// parseAspect parses a "W:H" string into an integer ratio (returns 0,0 on failure).
func parseAspect(aspectRatio string) (int, int) {
	a, b, ok := strings.Cut(strings.TrimSpace(aspectRatio), ":")
	if !ok {
		return 0, 0
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(a))
	h, err2 := strconv.Atoi(strings.TrimSpace(b))
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return w, h
}
