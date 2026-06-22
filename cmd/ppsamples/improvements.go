package main

// Demonstrates the before/after of the previous session's improvements
// (extract.go bodyExtent scaling, segment.go overlap recovery) using synthetic
// strips. It calls the sprite pipeline directly without any AI calls, and the
// "WITHOUT" case faithfully reproduces the old behavior so they can be shown
// side by side.

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"sort"

	"perfectpixel/internal/sprite"

	xdraw "golang.org/x/image/draw"
)

// contentColRuns returns the content column runs separated by empty columns (gap or more in a row).
func contentColRuns(strip *image.NRGBA, gap int) [][2]int {
	w, h := strip.Rect.Dx(), strip.Rect.Dy()
	colFull := make([]bool, w)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			if strip.Pix[strip.PixOffset(x, y)+3] > aThresh {
				colFull[x] = true
				break
			}
		}
	}
	var runs [][2]int
	x := 0
	for x < w {
		if !colFull[x] {
			x++
			continue
		}
		start := x
		empty := 0
		for x < w && empty < gap {
			if colFull[x] {
				empty = 0
			} else {
				empty++
			}
			x++
		}
		end := x - empty
		runs = append(runs, [2]int{start, end})
	}
	return runs
}

func runBBox(strip *image.NRGBA, x0, x1, h int) (minX, minY, maxX, maxY int) {
	minX, minY, maxX, maxY = x1, h, x0-1, -1
	for x := x0; x < x1; x++ {
		for y := 0; y < h; y++ {
			if strip.Pix[strip.PixOffset(x, y)+3] > aThresh {
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
	return
}

// bboxSharedExtract reproduces the old ExtractFrames behavior: it derives the
// global scale shared by all frames from the "largest bounding box" (= one
// frame's outstretched limb shrinks every frame together).
func bboxSharedExtract(strip *image.NRGBA, cellW, cellH, margin int) []*image.NRGBA {
	h := strip.Rect.Dy()
	runs := contentColRuns(strip, 6)
	type box struct{ minX, minY, maxX, maxY int }
	var boxes []box
	baseline := 0
	maxW, maxEffH := 1, 1
	for _, r := range runs {
		bx0, by0, bx1, by1 := runBBox(strip, r[0], r[1], h)
		if bx1 < bx0 {
			continue
		}
		boxes = append(boxes, box{bx0, by0, bx1, by1})
		if by1 > baseline {
			baseline = by1
		}
	}
	for _, b := range boxes {
		if w := b.maxX - b.minX + 1; w > maxW {
			maxW = w
		}
		if eff := (b.maxY - b.minY + 1) + (baseline - b.maxY); eff > maxEffH {
			maxEffH = eff
		}
	}
	availW, availH := cellW-margin*2, cellH-margin*2
	scale := float64(availW) / float64(maxW)
	if s := float64(availH) / float64(maxEffH); s < scale {
		scale = s
	}
	if scale > 1 {
		scale = 1
	}
	var frames []*image.NRGBA
	for _, b := range boxes {
		gw, gh := b.maxX-b.minX+1, b.maxY-b.minY+1
		sw, sh := int(float64(gw)*scale+0.5), int(float64(gh)*scale+0.5)
		src := image.NewNRGBA(image.Rect(0, 0, gw, gh))
		for y := b.minY; y <= b.maxY; y++ {
			for x := b.minX; x <= b.maxX; x++ {
				si := strip.PixOffset(x, y)
				if strip.Pix[si+3] > aThresh {
					di := src.PixOffset(x-b.minX, y-b.minY)
					copy(src.Pix[di:di+4], strip.Pix[si:si+4])
				}
			}
		}
		scaled := image.NewNRGBA(image.Rect(0, 0, sw, sh))
		xdraw.CatmullRom.Scale(scaled, scaled.Rect, src, src.Rect, xdraw.Over, nil)
		cell := newCanvas(cellW, cellH, color.NRGBA{0, 0, 0, 0})
		left := (cellW - sw) / 2
		offset := int(float64(baseline-b.maxY)*scale + 0.5)
		top := cellH - margin - offset - sh
		if top < 0 {
			top = 0
		}
		xdraw.Copy(cell, image.Point{X: left, Y: top}, scaled, scaled.Rect, xdraw.Over, nil)
		frames = append(frames, cell)
	}
	return frames
}

// bodyHeightPx is the height of the tallest opaque column in a frame (a measure of rendered character size).
func bodyHeightPx(f *image.NRGBA) int {
	w, h := f.Rect.Dx(), f.Rect.Dy()
	best := 0
	for x := 0; x < w; x++ {
		top, bot := -1, -1
		for y := 0; y < h; y++ {
			if f.Pix[f.PixOffset(x, y)+3] > aThresh {
				if top < 0 {
					top = y
				}
				bot = y
			}
		}
		if top >= 0 && bot-top+1 > best {
			best = bot - top + 1
		}
	}
	return best
}

func meanBodyHeight(frames []*image.NRGBA, skip int) float64 {
	sum, n := 0.0, 0
	for i, f := range frames {
		if i == skip {
			continue
		}
		sum += float64(bodyHeightPx(f))
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// scanBodyExtent compares, across all real strips, the normal-frame body height
// ratio of the old shared-bbox scale vs. the new ExtractFrames, to find the most
// dramatic candidate for the body-extent demo.
func scanBodyExtent() {
	strips, _ := filepath.Glob("sample/*/*/_strip.png")
	type cand struct {
		path  string
		n     int
		ratio float64
	}
	var cands []cand
	for _, p := range strips {
		dir := filepath.Dir(p)
		n := countFrames(dir)
		if n < 3 {
			continue
		}
		strip, err := loadPNG(p)
		if err != nil {
			continue
		}
		without := bboxSharedExtract(strip, cell, cell, 16)
		ext := sprite.ExtractFrames(strip, n, cell, cell, 16)
		if len(without) < 2 || ext.Found != n {
			continue
		}
		hWo := meanBodyHeight(without, -1)
		hWi := meanBodyHeight(ext.Frames, -1)
		if hWi <= 0 {
			continue
		}
		cands = append(cands, cand{p, n, hWo / hWi})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].ratio < cands[j].ratio })
	for i := 0; i < len(cands) && i < 12; i++ {
		fmt.Printf("  %-46s n=%d  bodyHeight WITHOUT/WITH=%.2f\n", cands[i].path, cands[i].n, cands[i].ratio)
	}
}
