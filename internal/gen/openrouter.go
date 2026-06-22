package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

const openRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouter is an image generation client that goes through OpenRouter.
type OpenRouter struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

// NewOpenRouter creates a new OpenRouter client.
func NewOpenRouter(apiKey, model string) *OpenRouter {
	if model == "" {
		model = DefaultModelFor(ProviderOpenRouter)
	}
	return &OpenRouter{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 180 * time.Second},
	}
}

type orContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *orImageURL `json:"image_url,omitempty"`
}

type orImageURL struct {
	URL string `json:"url"`
}

type orRequest struct {
	Model      string      `json:"model"`
	Messages   []orMessage `json:"messages"`
	Modalities []string    `json:"modalities"`
}

type orMessage struct {
	Role    string          `json:"role"`
	Content []orContentPart `json:"content"`
}

type orResponse struct {
	Choices []struct {
		Message struct {
			Images []struct {
				ImageURL orImageURL `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateImage generates an image with the OpenRouter chat completions API.
func (c *OpenRouter) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.APIKey == "" {
		return nil, errors.New("OpenRouter API key is not set. Please enter it in the settings")
	}

	// OpenRouter has no aspect ratio parameter, so we steer it through the prompt.
	fullPrompt := prompt + "\n\n" + aspectHint(aspectRatio)

	parts := []orContentPart{{Type: "text", Text: fullPrompt}}
	for _, img := range refImages {
		parts = append(parts, orContentPart{
			Type: "image_url",
			ImageURL: &orImageURL{
				URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(img),
			},
		})
	}

	body, err := json.Marshal(orRequest{
		Model:      c.Model,
		Messages:   []orMessage{{Role: "user", Content: parts}},
		Modalities: []string{"image", "text"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// During concurrent batch generation, transient 429 (rate limit) responses are common, so we
	// allow generous retries (6) and spread the load with capped exponential backoff plus jitter.
	var lastErr error
	backoff := 2 * time.Second
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Int63n(int64(time.Second)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
			if backoff < 16*time.Second {
				backoff *= 2
			}
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

func (c *OpenRouter) doRequest(ctx context.Context, body []byte) (img []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("X-Title", "PerfectPixel")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("failed to read response: %w", err)
	}

	var parsed orResponse
	_ = json.Unmarshal(respBytes, &parsed)

	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500
		if parsed.Error != nil {
			return nil, retryable, fmt.Errorf("OpenRouter error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("OpenRouter error (HTTP %d)", resp.StatusCode)
	}
	if parsed.Error != nil {
		return nil, true, fmt.Errorf("OpenRouter error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || len(parsed.Choices[0].Message.Images) == 0 {
		return nil, true, errors.New("the response contains no image")
	}
	data, err := decodeDataOrDownload(c.HTTP, parsed.Choices[0].Message.Images[0].ImageURL.URL)
	if err != nil {
		return nil, false, err
	}
	return data, false, nil
}

// ValidateKey checks whether the OpenRouter key is valid.
func (c *OpenRouter) ValidateKey(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/key", nil)
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
		return errors.New("the API key is invalid")
	}
	return fmt.Errorf("key validation failed (HTTP %d)", resp.StatusCode)
}
