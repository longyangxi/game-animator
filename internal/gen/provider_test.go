package gen

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewFactory(t *testing.T) {
	cases := []struct {
		provider string
		wantType string
	}{
		{"gemini", "*gen.Client"},
		{"", "*gen.Client"},
		{"openai", "*gen.OpenAI"},
		{"openrouter", "*gen.OpenRouter"},
		{"fal", "*gen.Fal"},
		{"byteplus", "*gen.BytePlus"},
	}
	for _, c := range cases {
		p, err := New(c.provider, "test-key", "")
		if err != nil {
			t.Fatalf("[%s] factory error: %v", c.provider, err)
		}
		if got := typeName(p); got != c.wantType {
			t.Fatalf("[%s] type error: got %s want %s", c.provider, got, c.wantType)
		}
	}
	if _, err := New("unknown", "k", ""); err == nil {
		t.Fatal("an unknown provider should be an error")
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *Client:
		return "*gen.Client"
	case *OpenAI:
		return "*gen.OpenAI"
	case *OpenRouter:
		return "*gen.OpenRouter"
	case *Fal:
		return "*gen.Fal"
	case *BytePlus:
		return "*gen.BytePlus"
	default:
		return "?"
	}
}

func TestDefaultModelFor(t *testing.T) {
	if DefaultModelFor("gemini") != DefaultModel {
		t.Fatal("gemini default model error")
	}
	if DefaultModelFor("openai") != "gpt-image-2" {
		t.Fatal("openai default model error")
	}
	if DefaultModelFor("openrouter") != "google/gemini-3-pro-image-preview" {
		t.Fatal("openrouter default model error")
	}
	if DefaultModelFor("fal") != "fal-ai/nano-banana-pro" {
		t.Fatal("fal default model error")
	}
	if DefaultModelFor("byteplus") != "seedream-4-0-250828" {
		t.Fatal("byteplus default model error")
	}
}

func TestModelsFor(t *testing.T) {
	for _, p := range SupportedProviders {
		models := ModelsFor(p)
		if len(models) == 0 {
			t.Fatalf("[%s] the model list is empty", p)
		}
		// The newest (first) model should match the default model.
		if models[0] != DefaultModelFor(p) {
			t.Fatalf("[%s] the newest model differs from the default model: %s != %s", p, models[0], DefaultModelFor(p))
		}
	}
	if ModelsFor("unknown") != nil {
		t.Fatal("an unknown provider should return nil")
	}
}

func TestBPSizeFor(t *testing.T) {
	if got := bpSizeFor("1:1"); got != "4096x4096" && got != "2048x2048" {
		t.Fatalf("square size error: %s", got)
	}
	if got := bpSizeFor(""); got != "2048x2048" {
		t.Fatalf("empty aspect ratio default size error: %s", got)
	}
	// Wide strip: long side 4096, short side clamped to a minimum of 1280.
	if got := bpSizeFor("7:1"); got != "4096x1280" {
		t.Fatalf("wide size clamp error: %s", got)
	}
}

func TestOpenAISizeFor(t *testing.T) {
	if got := openAISizeFor("1:1"); got != "1024x1024" {
		t.Fatalf("square size error: %s", got)
	}
	if got := openAISizeFor("16:9"); got != "1792x1008" {
		t.Fatalf("16:9 size error: %s", got)
	}
	if got := openAISizeFor("21:9"); got != "1792x768" {
		t.Fatalf("21:9 size error: %s", got)
	}
}

func TestFalEndpoint(t *testing.T) {
	c := NewFal("k", "fal-ai/nano-banana")
	if got := c.endpoint(false); got != "https://fal.run/fal-ai/nano-banana" {
		t.Fatalf("default endpoint error: %s", got)
	}
	if got := c.endpoint(true); got != "https://fal.run/fal-ai/nano-banana/edit" {
		t.Fatalf("edit endpoint error: %s", got)
	}
	// A model that already ends in /edit should not have it appended again.
	c.Model = "fal-ai/nano-banana/edit"
	if got := c.endpoint(true); got != "https://fal.run/fal-ai/nano-banana/edit" {
		t.Fatalf("failed to prevent duplicate edit suffix: %s", got)
	}
}

func TestDecodeDataOrDownload(t *testing.T) {
	payload := []byte("hello-png-bytes")

	// data: URL decoding
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(payload)
	got, err := decodeDataOrDownload(http.DefaultClient, dataURL)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("data URL decoding failed: %v %q", err, got)
	}

	// http URL download
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	got, err = decodeDataOrDownload(srv.Client(), srv.URL)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("download failed: %v %q", err, got)
	}

	// Invalid data URL
	if _, err := decodeDataOrDownload(http.DefaultClient, "data:image/png;hex,00"); err == nil {
		t.Fatal("a data URL missing base64 should be an error")
	}
}

func TestAspectHint(t *testing.T) {
	if h := aspectHint("1:1"); h != "Render on a square 1:1 canvas." {
		t.Fatalf("1:1 hint error: %s", h)
	}
	if h := aspectHint("21:9"); h == "" || !contains(h, "21:9") {
		t.Fatalf("wide hint error: %s", h)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
