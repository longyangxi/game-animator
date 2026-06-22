package main

import (
	"testing"
)

// TestSessionRoundTrip verifies the session save → restore → delete flow.
// HOME is isolated to a temporary directory so the real user session is not touched.
func TestSessionRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()

	// Empty string before any save
	if got := a.LoadSession(); got != "" {
		t.Fatalf("initial session is not empty: %q", got)
	}

	payload := `{"v":1,"character":{"name":"hero"},"cellSize":256,"states":[]}`
	if err := a.SaveSession(payload); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}
	if got := a.LoadSession(); got != payload {
		t.Fatalf("restored session mismatch: %q", got)
	}

	// Overwrite
	payload2 := `{"v":1,"character":{"name":"slime"},"cellSize":128,"states":[]}`
	if err := a.SaveSession(payload2); err != nil {
		t.Fatalf("failed to overwrite session: %v", err)
	}
	if got := a.LoadSession(); got != payload2 {
		t.Fatalf("overwritten session mismatch: %q", got)
	}

	// Empty string after delete; a duplicate delete must not error either
	if err := a.ClearSession(); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}
	if got := a.LoadSession(); got != "" {
		t.Fatalf("session still present after delete: %q", got)
	}
	if err := a.ClearSession(); err != nil {
		t.Fatalf("duplicate delete errored: %v", err)
	}
}
