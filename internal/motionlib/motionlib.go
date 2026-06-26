// Package motionlib provides validated pose-template sprite strips used as a
// generation reference image to drive animation motion quality. Templates are
// front-row crops of borrowed sheets (see README.md) and are an internal
// generation input only — never shipped in user output.
package motionlib

import (
	"embed"
	"sort"

	"perfectpixel/internal/sprite"
)

//go:embed templates/*.png
var templates embed.FS

// Template returns the embedded pose strip for a state name (its base keyword
// after stripping any direction suffix), or (nil, false) if uncovered.
func Template(stateName string) ([]byte, bool) {
	base := sprite.BaseStateName(stateName)
	data, err := templates.ReadFile("templates/" + base + ".png")
	if err != nil {
		return nil, false
	}
	return data, true
}

// Covered returns the sorted list of preset names that have a template.
func Covered() []string {
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > 4 && name[len(name)-4:] == ".png" {
			out = append(out, name[:len(name)-4])
		}
	}
	sort.Strings(out)
	return out
}
