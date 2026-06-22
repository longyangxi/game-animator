// Package sprite implements the image → animated sprite conversion pipeline.
package sprite

import "image"

// StateSpec defines a single animation state (idle, walk, etc.).
type StateSpec struct {
	Name   string `json:"name"`
	Frames int    `json:"frames"`
	FPS    int    `json:"fps"`
	Loop   bool   `json:"loop"`
	Action string `json:"action"`
	Facing string `json:"facing"` // 8-direction key (south, etc.; empty means no direction instruction)
}

// ExtractResult is the result of extracting frames from a strip.
type ExtractResult struct {
	Frames   []*image.NRGBA
	Found    int
	Expected int
	Warnings []string
}

// StateFrames holds the final per-state frames that go into atlas composition.
type StateFrames struct {
	Spec   StateSpec
	Frames []*image.NRGBA
}

// FrameRect is a frame's coordinates within the sheet.
type FrameRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Point is a 2D integer coordinate (for pivots/anchors).
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// AnimationEntry is a per-state entry in the manifest.
// Rects are cell coordinates, Trims are the in-cell content bbox (local coordinates), and Pivot is the shared foot anchor.
type AnimationEntry struct {
	Row        int         `json:"row"`
	Frames     int         `json:"frames"`
	FPS        int         `json:"fps"`
	Loop       bool        `json:"loop"`
	DurationMs int         `json:"durationMs"` // display time per frame
	Pivot      Point       `json:"pivot"`      // cell-local foot anchor (bottom center)
	Rects      []FrameRect `json:"rects"`      // sheet absolute coordinates
	Trims      []FrameRect `json:"trims"`      // cell-local content bbox
}

// Manifest is the runtime spritesheet metadata (schema v2).
type Manifest struct {
	App        string                    `json:"app"`
	Generator  string                    `json:"generator"`
	Schema     string                    `json:"schema"`
	Version    int                       `json:"version"`
	Character  string                    `json:"character"`
	Sheet      SheetInfo                 `json:"sheet"`
	Animations map[string]AnimationEntry `json:"animations"`
}

// SheetInfo holds the sheet image information.
type SheetInfo struct {
	Image      string `json:"image"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	CellWidth  int    `json:"cellWidth"`
	CellHeight int    `json:"cellHeight"`
}
