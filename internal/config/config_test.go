package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	content := "# comment\n" +
		"FAL_KEY=abc123:secret\n" +
		"export OPENROUTER_API_KEY=\"sk-or-test\"\n" +
		"OPENAI_API_KEY='sk-openai-test'\n" +
		"GEMINI_API_KEY='AIza-test'\n" +
		"INVALID_LINE\n" +
		"EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out := map[string]string{}
	parseEnvFile(path, out)

	if out["FAL_KEY"] != "abc123:secret" {
		t.Fatalf("FAL_KEY parsing failed: %q", out["FAL_KEY"])
	}
	if out["OPENROUTER_API_KEY"] != "sk-or-test" {
		t.Fatalf("export + quotes parsing failed: %q", out["OPENROUTER_API_KEY"])
	}
	if out["OPENAI_API_KEY"] != "sk-openai-test" {
		t.Fatalf("OPENAI_API_KEY parsing failed: %q", out["OPENAI_API_KEY"])
	}
	if out["GEMINI_API_KEY"] != "AIza-test" {
		t.Fatalf("single-quote parsing failed: %q", out["GEMINI_API_KEY"])
	}
	if _, ok := out["EMPTY"]; ok {
		t.Fatal("empty values should be ignored")
	}
}

func TestSettingsCfg(t *testing.T) {
	s := Settings{
		Gemini:     ProviderCfg{APIKey: "g"},
		OpenAI:     ProviderCfg{APIKey: "ai"},
		OpenRouter: ProviderCfg{APIKey: "o"},
		Fal:        ProviderCfg{APIKey: "f"},
	}
	if s.Cfg("openai").APIKey != "ai" || s.Cfg("openrouter").APIKey != "o" || s.Cfg("fal").APIKey != "f" {
		t.Fatal("per-provider config mapping error")
	}
	// An unknown provider falls back to gemini.
	if s.Cfg("unknown").APIKey != "g" {
		t.Fatal("default fallback error")
	}
	// Because a pointer is returned, modifications should be reflected.
	s.Cfg("fal").Model = "m"
	if s.Fal.Model != "m" {
		t.Fatal("Cfg should return a pointer")
	}
}

func TestMotionLibraryEnabled(t *testing.T) {
	var s Settings
	if !s.MotionLibraryEnabled() {
		t.Errorf("unset MotionLibrary should default to enabled (true)")
	}
	off := false
	s.MotionLibrary = &off
	if s.MotionLibraryEnabled() {
		t.Errorf("explicit false should be disabled")
	}
	on := true
	s.MotionLibrary = &on
	if !s.MotionLibraryEnabled() {
		t.Errorf("explicit true should be enabled")
	}
}
