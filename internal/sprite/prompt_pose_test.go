package sprite

import (
	"strings"
	"testing"
)

func TestBaseStateName(t *testing.T) {
	cases := map[string]string{
		"attack":            "attack",
		"Attack":            "attack",
		"  attack ":         "attack",
		"attack-south":      "attack",
		"attack-north-east": "attack",
		"walk":              "walk",
	}
	for in, want := range cases {
		if got := BaseStateName(in); got != want {
			t.Errorf("BaseStateName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPoseTemplateClause(t *testing.T) {
	got := PoseTemplateClause("attack", 5)
	for _, want := range []string{
		"Pose template (CRITICAL",
		"Image 1 is the CANONICAL CHARACTER",
		"Image 2 is a POSE TEMPLATE",
		`"attack" action`,
		"EXACTLY 5 poses",
		"Do NOT reproduce any glow, slash-arc, swoosh",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PoseTemplateClause missing %q\n--- got ---\n%s", want, got)
		}
	}
}
