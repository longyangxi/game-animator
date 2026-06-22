package gen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Stand-in payload for a 1px PNG (decoding is not validated, so arbitrary bytes suffice).
var fakePNG = []byte{0x89, 'P', 'N', 'G', 1, 2, 3}

func imageResponse() string {
	b64 := base64.StdEncoding.EncodeToString(fakePNG)
	return fmt.Sprintf(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":%q}}]},"finishReason":"STOP"}]}`, b64)
}

// TestGeminiModelFallback verifies that a 404 on the default model automatically switches to the fallback chain.
func TestGeminiModelFallback(t *testing.T) {
	var calledModels []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path format: /models/<model>:generateContent
		path := strings.TrimPrefix(r.URL.Path, "/models/")
		model := strings.TrimSuffix(path, ":generateContent")
		calledModels = append(calledModels, model)
		if model == DefaultModel || model == "gemini-3-pro-image-preview" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"model not found","status":"NOT_FOUND"}}`))
			return
		}
		_, _ = w.Write([]byte(imageResponse()))
	}))
	defer srv.Close()

	c := NewClient("test-key", "")
	c.endpoint = srv.URL + "/models/%s:generateContent"

	img, err := c.GenerateImage(context.Background(), "prompt", nil, "1:1")
	if err != nil {
		t.Fatalf("fallback generation failed: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image bytes mismatch")
	}
	want := []string{DefaultModel, "gemini-3-pro-image-preview", "gemini-3.1-flash-image"}
	if len(calledModels) != len(want) {
		t.Fatalf("called model sequence error: %v", calledModels)
	}
	for i := range want {
		if calledModels[i] != want[i] {
			t.Fatalf("call order error: got %v want %v", calledModels, want)
		}
	}
}

// TestGeminiAllModelsNotFound verifies that an error is returned once all fallbacks are exhausted.
func TestGeminiAllModelsNotFound(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"nope","status":"NOT_FOUND"}}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", "")
	c.endpoint = srv.URL + "/models/%s:generateContent"
	if _, err := c.GenerateImage(context.Background(), "p", nil, ""); err == nil {
		t.Fatal("a 404 on all models should be an error")
	}
	// default + 3 fallbacks = 4 calls
	if calls != 1+len(modelFallbacks) {
		t.Fatalf("call count error: %d", calls)
	}
}

func TestGeminiTimeoutIsNotRetried(t *testing.T) {
	calls := 0
	c := NewClient("test-key", "")
	c.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, context.DeadlineExceeded
	})}

	_, err := c.GenerateImage(context.Background(), "p", nil, "1:1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a deadline error but got %v", err)
	}
	if calls != 1 {
		t.Fatalf("the timed-out request was called %d times; it must not be retried", calls)
	}
}

// TestGeminiNonDefaultModelFallback verifies that the fallback chain still works
// when a user-specified model returns 404.
func TestGeminiNonDefaultModelFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "my-custom-model") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"x","status":"NOT_FOUND"}}`))
			return
		}
		_, _ = w.Write([]byte(imageResponse()))
	}))
	defer srv.Close()

	c := NewClient("test-key", "my-custom-model")
	c.endpoint = srv.URL + "/models/%s:generateContent"
	if _, err := c.GenerateImage(context.Background(), "p", nil, ""); err != nil {
		t.Fatalf("user model fallback failed: %v", err)
	}
}

// TestGeminiRequestBody verifies that the reference image and aspect ratio are placed correctly in the request.
func TestGeminiRequestBody(t *testing.T) {
	var got genRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(imageResponse()))
	}))
	defer srv.Close()

	c := NewClient("test-key", "")
	c.endpoint = srv.URL + "/models/%s:generateContent"
	if _, err := c.GenerateImage(context.Background(), "hello", [][]byte{fakePNG}, "21:9"); err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if len(got.Contents) != 1 || len(got.Contents[0].Parts) != 2 {
		t.Fatalf("parts composition error: %+v", got)
	}
	if got.Contents[0].Parts[0].InlineData == nil || got.Contents[0].Parts[1].Text != "hello" {
		t.Fatal("the reference image should come before the prompt")
	}
	if got.GenerationConfig == nil || got.GenerationConfig.ImageConfig == nil ||
		got.GenerationConfig.ImageConfig.AspectRatio != "21:9" {
		t.Fatalf("aspectRatio missing: %+v", got.GenerationConfig)
	}
	// Gemini 3 Pro + wide strip -> 2K resolution to secure pixels per frame.
	if got.GenerationConfig.ImageConfig.ImageSize != "2K" {
		t.Fatalf("imageSize 2K missing: %+v", got.GenerationConfig.ImageConfig)
	}
}

// TestGeminiImageSizeOnlyForPro verifies that imageSize is omitted from fallback model requests.
func TestGeminiImageSizeOnlyForPro(t *testing.T) {
	sizeByModel := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/models/")
		model := strings.TrimSuffix(path, ":generateContent")
		var req genRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.GenerationConfig != nil && req.GenerationConfig.ImageConfig != nil {
			sizeByModel[model] = req.GenerationConfig.ImageConfig.ImageSize
		}
		if strings.HasPrefix(model, "gemini-3-pro") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"x","status":"NOT_FOUND"}}`))
			return
		}
		_, _ = w.Write([]byte(imageResponse()))
	}))
	defer srv.Close()

	c := NewClient("test-key", "")
	c.endpoint = srv.URL + "/models/%s:generateContent"
	if _, err := c.GenerateImage(context.Background(), "p", nil, "21:9"); err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if sizeByModel[DefaultModel] != "2K" {
		t.Fatalf("2K missing for the Pro model: %v", sizeByModel)
	}
	if sizeByModel["gemini-3.1-flash-image"] != "" {
		t.Fatalf("the fallback model included imageSize: %v", sizeByModel)
	}
}
