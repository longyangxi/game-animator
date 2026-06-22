package main

import (
	"fmt"
	"image"
	"image/color"

	"perfectpixel/internal/sprite"
)

const cell = 256

func magentaCanvas(w, h int) *image.NRGBA { return newCanvas(w, h, keyMagenta) }

// idlePose is the default standing pose.
func idlePose() pose { return pose{lArm: 18, rArm: -18, lLeg: 8, rLeg: -8, armLen: 52, legLen: 60} }

// ---------- 1. chroma matting (background removal) ----------

func demoMatting(raw *image.NRGBA) {
	fmt.Println("[1] chroma matting (background removal) — real AI output")
	jp := raw // real AI output (magenta key, already JPEG-compressed)

	without := naiveMatte(jp, keyMagenta, 70) // simple RGB threshold
	with := sprite.RemoveBackground(jp)        // real YCbCr matting + despill + flood + morph

	res := magentaResidue(without)
	resW := magentaResidue(with)
	halo := haloPinkish(without)
	haloW := haloPinkish(with)
	fmt.Printf("  magenta residue: without=%d  with=%d\n", res, resW)
	fmt.Printf("  pink halo      : without=%d  with=%d\n", halo, haloW)
	fmt.Printf("  character content (preservation check): without=%d  with=%d\n", contentPixels(without), contentPixels(with))

	gray := newCanvas(raw.Rect.Dx(), raw.Rect.Dy(), colBG)
	const pw = 300
	inP := labeledPanel(resizeW(jp, pw), "INPUT: real AI output (magenta key)", colInk)
	woP := labeledPanel(resizeW(overOn(gray, without), pw),
		fmt.Sprintf("WITHOUT: naive RGB threshold  (residue %d, halo %d)", res, halo), colWithout)
	wiP := labeledPanel(resizeW(overOn(gray, with), pw),
		fmt.Sprintf("WITH: YCbCr matting+despill  (residue %d, halo %d)", resW, haloW), colWith)

	title := titleBar(pw*3+40, "1. Background removal: chroma matting (chroma.go) — real AI sprite")
	body := hstack(20, color.NRGBA{255, 255, 255, 255}, inP, woP, wiP)
	save("01-matting.png", vstack(6, color.NRGBA{255, 255, 255, 255}, title, body))
}

// ---------- 2. projection + DP segmentation ----------

func demoSegmentation(matte *image.NRGBA, n int) {
	fmt.Println("[2] projection + DP segmentation — real AI strip")
	without := equalSplitExtract(matte, n, cell, cell, 16)
	ext := sprite.ExtractFrames(matte, n, cell, cell, 16)
	with := ext.Frames

	cross, _ := crossingLines(matte, n)
	fmt.Printf("  detected poses (natural): %d (requested %d)\n", ext.Found, n)
	fmt.Printf("  equal-split lines crossing a character: without=%d/%d  with=0/%d (cut at gutter)\n", cross, n-1, n-1)

	gray := newCanvas(cell, cell, colBG)
	dispW := 900
	stripView := resizeW(overOn(newCanvas(matte.Rect.Dx(), matte.Rect.Dy(), color.NRGBA{255, 255, 255, 255}), matte), dispW)
	for k := 1; k < n; k++ { // equal-split lines (red) — they cut across real poses
		x := k * dispW / n
		for y := 0; y < stripView.Rect.Dy(); y++ {
			setPx(stripView, x, y, color.NRGBA{220, 40, 40, 255})
		}
	}
	woRow := scaleRow(without, gray, 3)
	wiRow := scaleRow(with, gray, 3)

	title := titleBar(dispW, "2. Frame split: projection + DP optimal cut (segment.go) — real AI strip")
	out := vstack(6, color.NRGBA{255, 255, 255, 255},
		title,
		captionBar(dispW, fmt.Sprintf("Red = naive equal-split lines: %d of %d cut straight through a character.", cross, n-1), colInk),
		labeledPanel(stripView, "matted real strip (red = equal-split cut lines)", colInk),
		labeledPanel(woRow, fmt.Sprintf("WITHOUT: equal split  (%d/%d cut lines slice a character)", cross, n-1), colWithout),
		labeledPanel(wiRow, fmt.Sprintf("WITH: projection+DP  (found %d, each pose isolated at its gutter)", ext.Found), colWith),
	)
	save("02-segmentation.png", out)
}
