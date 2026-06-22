package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	openAIImageGenerationEndpoint = "https://api.openai.com/v1/images/generations"
	openAIImageEditEndpoint       = "https://api.openai.com/v1/images/edits"
	openAIModelsEndpoint          = "https://api.openai.com/v1/models"
)

// OpenAI is a client for the OpenAI Image API.
type OpenAI struct {
	APIKey string
	Model  string
	HTTP   *http.Client

	generationEndpoint string // test override
	editEndpoint       string // test override
	modelsEndpoint     string // test override
}

// NewOpenAI creates a new OpenAI image generation client.
func NewOpenAI(apiKey, model string) *OpenAI {
	if model == "" {
		model = DefaultModelFor(ProviderOpenAI)
	}
	return &OpenAI{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 300 * time.Second},
	}
}

type openAIImageRequest struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	N            int    `json:"n"`
	Size         string `json:"size,omitempty"`
	Quality      string `json:"quality,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

type openAIImageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// GenerateImage generates an image with the OpenAI GPT Image model, or edits reference images.
func (c *OpenAI) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("OpenAI API key is not set. Please enter it in the settings")
	}

	fullPrompt := prompt + "\n\n" + aspectHint(aspectRatio)

	endpoint := c.generationEndpoint
	if len(refImages) > 0 {
		endpoint = c.editEndpoint
		if endpoint == "" {
			endpoint = openAIImageEditEndpoint
		}
	} else if endpoint == "" {
		endpoint = openAIImageGenerationEndpoint
	}

	buildBody := func(size string) ([]byte, string, error) {
		if len(refImages) == 0 {
			b, err := json.Marshal(openAIImageRequest{
				Model:        c.Model,
				Prompt:       fullPrompt,
				N:            1,
				Size:         size,
				Quality:      "medium",
				OutputFormat: "png",
			})
			return b, "application/json", err
		}
		return c.buildEditBody(fullPrompt, refImages, size)
	}

	// Try the frame-scaled size first; if the model rejects it (a non-retryable error, usually a
	// size the API won't accept), fall back once to the proven-good 1792x768 instead of failing.
	const safeSize = "1792x768"
	sizes := []string{openAISizeFor(aspectRatio)}
	if sizes[0] != safeSize {
		sizes = append(sizes, safeSize)
	}

	var lastErr error
	for si, size := range sizes {
		body, contentType, err := buildBody(size)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request: %w", err)
		}
		backoff := 2 * time.Second
		nonRetryable := false
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
			}
			img, retryable, err := c.doImageRequest(ctx, endpoint, contentType, body)
			if err == nil {
				return img, nil
			}
			lastErr = err
			if !retryable {
				nonRetryable = true
				break
			}
		}
		// On a non-retryable failure, try the next (fallback) size if there is one.
		if nonRetryable && si == len(sizes)-1 {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func (c *OpenAI) buildEditBody(prompt string, refImages [][]byte, size string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{
		"model":         c.Model,
		"prompt":        prompt,
		"n":             "1",
		"size":          size,
		"quality":       "medium",
		"output_format": "png",
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	for i, img := range refImages {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="reference-%d.png"`, i+1))
		h.Set("Content-Type", "image/png")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(img); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func (c *OpenAI) doImageRequest(ctx context.Context, endpoint, contentType string, body []byte) (img []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("failed to read response: %w", err)
	}

	var parsed openAIImageResponse
	_ = json.Unmarshal(respBytes, &parsed)

	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500
		if parsed.Error != nil && parsed.Error.Message != "" {
			return nil, retryable, fmt.Errorf("OpenAI error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("OpenAI error (HTTP %d)", resp.StatusCode)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, true, fmt.Errorf("OpenAI error: %s", parsed.Error.Message)
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

// ValidateKey checks whether the OpenAI key is valid.
func (c *OpenAI) ValidateKey(ctx context.Context) error {
	key := strings.TrimSpace(c.APIKey)
	if !strings.HasPrefix(key, "sk-") || len(key) < 20 {
		return errors.New("the OpenAI API key format is invalid")
	}
	ep := c.modelsEndpoint
	if ep == "" {
		ep = openAIModelsEndpoint + "/" + c.Model
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return errors.New("the API key is invalid or you do not have access to this model")
	}
	if resp.StatusCode == 404 {
		return errors.New("the GPT Image model could not be found on this OpenAI account")
	}
	return fmt.Errorf("key validation failed (HTTP %d)", resp.StatusCode)
}

// openAISizeFor converts an aspect ratio into a WxH resolution allowed by GPT Image 2.
func openAISizeFor(aspectRatio string) string {
	w, h := parseAspect(aspectRatio)
	if w <= 0 || h <= 0 {
		return "1024x1024"
	}
	if w == h {
		return "1024x1024"
	}
	// Scale the long edge with the aspect ratio so a wider multi-pose strip gets proportionally
	// more total width (each pose keeps a usable pixel budget) instead of being squeezed into a
	// fixed long edge. The short edge stays ~rowHeight, so cost scales with pose count rather than
	// a flat maximum. Floored at 1792 (square-ish/small strips are unchanged from before) and
	// capped at 4096 (gpt-image-2's maximum).
	const rowHeight = 768.0
	const minEdge, maxEdge = 1792, 4096
	ratio := float64(w) / float64(h)
	if ratio < 1 {
		ratio = 1 / ratio
	}
	longEdge := int(rowHeight*ratio + 0.5)
	if longEdge < minEdge {
		longEdge = minEdge
	}
	if longEdge > maxEdge {
		longEdge = maxEdge
	}
	var pw, ph int
	if w > h {
		pw = longEdge
		ph = int(float64(longEdge) * float64(h) / float64(w))
	} else {
		ph = longEdge
		pw = int(float64(longEdge) * float64(w) / float64(h))
	}
	round16 := func(v int) int {
		if v < 640 {
			v = 640
		}
		return (v / 16) * 16
	}
	return strconv.Itoa(round16(pw)) + "x" + strconv.Itoa(round16(ph))
}
