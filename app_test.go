package main

import (
	"testing"
)

func TestLibraryTemplate(t *testing.T) {
	// enabled, covered action, no RefStrip -> template returned, clause on
	tpl, use := libraryTemplate("attack", false, true)
	if !use || len(tpl) == 0 {
		t.Fatalf("attack/enabled/no-refstrip should use the library template")
	}
	// disabled -> never
	if _, use := libraryTemplate("attack", false, false); use {
		t.Errorf("disabled toggle must not use the library")
	}
	// RefStrip present (directional set) -> never
	if _, use := libraryTemplate("attack", true, true); use {
		t.Errorf("RefStrip precedence: must not use the library")
	}
	// uncovered action -> never
	if _, use := libraryTemplate("taunt", false, true); use {
		t.Errorf("uncovered action must not use the library")
	}
}

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
