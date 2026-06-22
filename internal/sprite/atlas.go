package sprite

import (
	"image"

	xdraw "golang.org/x/image/draw"
)

// ComposeAtlas builds a spritesheet with per-state frames laid out row by row, plus a manifest.
func ComposeAtlas(character string, states []StateFrames, cellW, cellH int) (*image.NRGBA, Manifest) {
	maxFrames := 1
	for _, s := range states {
		if len(s.Frames) > maxFrames {
			maxFrames = len(s.Frames)
		}
	}
	sheetW := maxFrames * cellW
	sheetH := len(states) * cellH
	sheet := image.NewNRGBA(image.Rect(0, 0, sheetW, sheetH))

	manifest := Manifest{
		App:       "perfectpixel",
		Generator: "perfectpixel/component-lane",
		Schema:    "perfectpixel.sprite/2",
		Version:   2,
		Character: character,
		Sheet: SheetInfo{
			Image:      "sprite-sheet.png",
			Width:      sheetW,
			Height:     sheetH,
			CellWidth:  cellW,
			CellHeight: cellH,
		},
		Animations: map[string]AnimationEntry{},
	}

	for row, s := range states {
		fps := s.Spec.FPS
		if fps <= 0 {
			fps = 8
		}
		entry := AnimationEntry{
			Row:        row,
			Frames:     len(s.Frames),
			FPS:        fps,
			Loop:       s.Spec.Loop,
			DurationMs: 1000 / fps,
		}
		groundY := 0
		for col, frame := range s.Frames {
			x, y := col*cellW, row*cellH
			xdraw.Copy(sheet, image.Point{X: x, Y: y}, frame, frame.Rect, xdraw.Over, nil)
			entry.Rects = append(entry.Rects, FrameRect{X: x, Y: y, W: cellW, H: cellH})
			trim := contentBBox(frame)
			entry.Trims = append(entry.Trims, trim)
			if b := trim.Y + trim.H; b > groundY {
				groundY = b
			}
		}
		// Shared foot anchor: cell horizontal center + the lowest point (ground) of all frames' content.
		// However, if groundY is too high (content at the top), fall back to the default cell bottom
		// so the pivot does not poke outside the frame.
		if groundY < cellH/2 && len(s.Frames) > 0 {
			groundY = cellH
		}
		// abnormal pitfall: clamp so groundY does not extend below the cell
		if groundY > cellH {
			groundY = cellH
		}
		entry.Pivot = Point{X: cellW / 2, Y: groundY}
		manifest.Animations[s.Spec.Name] = entry
	}
	return sheet, manifest
}

// contentBBox computes the bounding rectangle of a frame's opaque content (cell-local coordinates).
func contentBBox(f *image.NRGBA) FrameRect {
	w, h := f.Rect.Dx(), f.Rect.Dy()
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if f.Pix[f.PixOffset(x, y)+3] > alphaThreshold {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX {
		return FrameRect{}
	}
	return FrameRect{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}
}
