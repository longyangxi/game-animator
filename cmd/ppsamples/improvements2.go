package main

import (
	"fmt"
	"image"
	"image/color"

	"perfectpixel/internal/sprite"
)

// buildOverview vertically stitches the saved before/after comparison PNGs into
// a single contact sheet, so the fixes can be seen at a glance.
func buildOverview() {
	files := []string{
		"01-matting.png", "02-segmentation.png", "03-centroid.png",
		"04-pixelize.png", "05-bodyextent.png", "06-overlap.png",
	}
	const colW = 920
	white := color.NRGBA{255, 255, 255, 255}
	panels := []*image.NRGBA{
		titleBar(colW, "PerfectPixel sprite pipeline — before / after at a glance (red = WITHOUT, green = WITH)"),
	}
	for _, f := range files {
		im, err := loadPNG("report-images/" + f)
		if err != nil {
			continue
		}
		if im.Rect.Dx() > colW {
			im = resizeW(im, colW)
		}
		panels = append(panels, im)
	}
	out := vstack(14, white, panels...)
	save("00-overview.png", out)
}

// transparentStrip builds an empty strip with an alpha-0 background (already background-removed).
func transparentStrip(w, h int) *image.NRGBA {
	return newCanvas(w, h, color.NRGBA{0, 0, 0, 0})
}

// demoBodyExtent demonstrates how body-extent scaling (extract.go) prevents the
// problem where one frame's outstretched weapon/arm dominated the overall scale
// and shrank every frame, using a real AI strip (a set mixing in a sword-thrust
// slash pose).
func demoBodyExtent() {
	fmt.Println("[5] body-extent scaling (extract.go) — outstretched-weapon outlier in a real AI strip")
	strip, n := pickSampleStrip("sample/archer/slash-south-east", 5)

	without := bboxSharedExtract(strip, cell, cell, 16)
	ext := sprite.ExtractFrames(strip, n, cell, cell, 16)
	with := ext.Frames

	hWo := meanBodyHeight(without, -1)
	hWi := meanBodyHeight(with, -1)
	ratio := 0.0
	if hWi > 0 {
		ratio = hWo / hWi
	}
	fmt.Printf("  mean body height: without=%.0fpx  with=%.0fpx  (without shrinks to %.0f%%)\n",
		hWo, hWi, ratio*100)

	gray := newCanvas(cell, cell, colBG)
	woRow := scaleRow(without, gray, 3)
	wiRow := scaleRow(with, gray, 3)
	dispW := woRow.Rect.Dx()
	if wiRow.Rect.Dx() > dispW {
		dispW = wiRow.Rect.Dx()
	}
	out := vstack(6, color.NRGBA{255, 255, 255, 255},
		titleBar(dispW, "5. Frame scale: body-extent vs bounding-box (extract.go) — real AI strip"),
		captionBar(dispW, "One pose thrusts a sword far out. Old global scale shrinks EVERY frame to fit that bbox.", colInk),
		labeledPanel(woRow, fmt.Sprintf("WITHOUT: bbox global scale — characters shrink to %.0f%% (outstretched weapon dominates)", ratio*100), colWithout),
		labeledPanel(wiRow, "WITH: body-extent scale — 80% alpha mass drives scale, character size stays consistent", colWith),
	)
	save("05-bodyextent.png", out)
}

// tightCrop returns a new image cropped to the content bounding box.
func tightCrop(im *image.NRGBA) *image.NRGBA {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
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
		return cloneNRGBA(im)
	}
	return cropRect(im, minX, minY, maxX-minX+1, maxY-minY+1)
}

// loadFrame reads frame-NN.png from the sample directory (the final extracted single pose).
func loadFrame(dir string, idx int) *image.NRGBA {
	im, err := loadPNG(fmt.Sprintf("%s/frame-%02d.png", dir, idx))
	if err != nil {
		return nil
	}
	return im
}

// demoOverlapRecovery abuts two real AI poses with no magenta gutter into a
// single blob, then demonstrates whether segment.go's forced split recovers
// them back into two.
func demoOverlapRecovery() {
	fmt.Println("[6] overlap segmentation recovery (segment.go) — real poses abutted with no gutter")
	const n = 2
	dir := "sample/ranger/walk"
	a, b := loadFrame(dir, 2), loadFrame(dir, 3)
	if a == nil || b == nil {
		fmt.Println("  [warning] no real frames -> skipping demo")
		return
	}
	a, b = tightCrop(a), tightCrop(b)
	// Place the two poses on one strip, bottom-aligned, overlapping by just 8px (no gutter).
	pad := 24
	overlap := 8
	stripW := pad + a.Rect.Dx() + b.Rect.Dx() - overlap + pad
	stripH := a.Rect.Dy()
	if b.Rect.Dy() > stripH {
		stripH = b.Rect.Dy()
	}
	stripH += 16
	strip := transparentStrip(stripW, stripH)
	paste(strip, a, pad, stripH-16-a.Rect.Dy())
	paste(strip, b, pad+a.Rect.Dx()-overlap, stripH-16-b.Rect.Dy())

	ext := sprite.ExtractFrames(strip, n, cell, cell, 16)
	with := ext.Frames
	merged := bboxSharedExtract(strip, cell, cell, 16)

	fmt.Printf("  detected poses: without=%d(merged)  with=%d(recovered)\n", len(merged), ext.Found)

	gray := newCanvas(cell, cell, colBG)
	woRow := scaleRow(merged, gray, 3)
	wiRow := scaleRow(with, gray, 3)
	stripView := resizeW(overOn(newCanvas(strip.Rect.Dx(), strip.Rect.Dy(), colBG), strip), 360)
	dispW := wiRow.Rect.Dx()
	for _, d := range []int{woRow.Rect.Dx(), stripView.Rect.Dx()} {
		if d > dispW {
			dispW = d
		}
	}
	out := vstack(6, color.NRGBA{255, 255, 255, 255},
		titleBar(dispW, "6. Overlap recovery: forced expected split (segment.go) — real AI poses"),
		captionBar(dispW, "Two real poses abutted with NO magenta gutter (AI sometimes draws them touching).", colInk),
		labeledPanel(stripView, "input: two poses merged into one blob (no gutter)", colInk),
		labeledPanel(woRow, fmt.Sprintf("WITHOUT: single peak read as %d pose (both bodies merged)", len(merged)), colWithout),
		labeledPanel(wiRow, fmt.Sprintf("WITH: DP forces expected count → %d clean poses recovered", ext.Found), colWith),
	)
	save("06-overlap.png", out)
}
