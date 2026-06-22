// Command ppsamples generates "before/after technique applied" comparison
// images for technical analysis reports using the real sprite pipeline code.
// Without any AI calls, it feeds a synthetic magenta strip as input and shows
// each algorithm enabled vs. disabled side by side.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"perfectpixel/internal/sprite"
)

const outDir = "report-images"

func save(name string, im image.Image) {
	p := filepath.Join(outDir, name)
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, im); err != nil {
		panic(err)
	}
	fmt.Printf("  saved %s (%dx%d)\n", p, im.Bounds().Dx(), im.Bounds().Dy())
}

// labeledPanel builds a panel with a colored label bar above the image.
// If the label is wider than the image, the bar is widened and the image is
// centered (to avoid clipping the text).
func labeledPanel(im *image.NRGBA, label string, bar color.NRGBA) *image.NRGBA {
	w := textWidth(label, im.Rect.Dx())
	barH := 26
	out := newCanvas(w, im.Rect.Dy()+barH, color.NRGBA{255, 255, 255, 255})
	fillRect(out, 0, 0, w, barH, bar)
	drawText(out, 8, 17, label, color.NRGBA{255, 255, 255, 255})
	paste(out, im, (w-im.Rect.Dx())/2, barH)
	return out
}

// hstack joins panels horizontally (with the given gap between them).
func hstack(gap int, bg color.NRGBA, panels ...*image.NRGBA) *image.NRGBA {
	w, h := 0, 0
	for i, p := range panels {
		w += p.Rect.Dx()
		if i > 0 {
			w += gap
		}
		if p.Rect.Dy() > h {
			h = p.Rect.Dy()
		}
	}
	out := newCanvas(w, h, bg)
	x := 0
	for _, p := range panels {
		paste(out, p, x, 0)
		x += p.Rect.Dx() + gap
	}
	return out
}

// vstack joins panels vertically.
func vstack(gap int, bg color.NRGBA, panels ...*image.NRGBA) *image.NRGBA {
	w, h := 0, 0
	for i, p := range panels {
		h += p.Rect.Dy()
		if i > 0 {
			h += gap
		}
		if p.Rect.Dx() > w {
			w = p.Rect.Dx()
		}
	}
	out := newCanvas(w, h, bg)
	y := 0
	for _, p := range panels {
		paste(out, p, 0, y)
		y += p.Rect.Dy() + gap
	}
	return out
}

// textWidth estimates the pixel width of text (including padding), based on
// basicfont (7px fixed width).
func textWidth(s string, minW int) int {
	w := len(s)*7 + 16
	if w < minW {
		return minW
	}
	return w
}

// titleBar builds a single-line title text panel (auto-sizing the width so the
// text isn't clipped).
func titleBar(w int, s string) *image.NRGBA {
	im := newCanvas(textWidth(s, w), 30, color.NRGBA{255, 255, 255, 255})
	drawText(im, 8, 20, s, colInk)
	return im
}

func captionBar(w int, s string, c color.NRGBA) *image.NRGBA {
	im := newCanvas(textWidth(s, w), 22, color.NRGBA{255, 255, 255, 255})
	drawText(im, 8, 15, s, c)
	return im
}

// frameRow builds a row laying frames out horizontally (including cell borders).
func frameRow(frames []*image.NRGBA, over *image.NRGBA, centerLine bool) *image.NRGBA {
	if len(frames) == 0 {
		return newCanvas(64, 64, color.NRGBA{255, 255, 255, 255})
	}
	cw, ch := frames[0].Rect.Dx(), frames[0].Rect.Dy()
	row := newCanvas(cw*len(frames), ch, color.NRGBA{255, 255, 255, 255})
	for i, f := range frames {
		cell := overOn(over, f)
		paste(row, cell, i*cw, 0)
		// cell border
		fillRect(row, i*cw, 0, i*cw+1, ch, color.NRGBA{180, 184, 190, 255})
		if centerLine {
			cx := i*cw + cw/2
			for y := 0; y < ch; y += 4 {
				setPx(row, cx, y, color.NRGBA{220, 40, 40, 255})
			}
		}
	}
	return row
}

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		scanSamples()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "scan2" {
		scanBodyExtent()
		return
	}
	fmt.Println("== PerfectPixel technique comparison image generation (real AI pipeline) ==")
	// Matting & pixelization: a freshly generated magenta base character (real AI)
	base, err := buildRealBase()
	if err != nil {
		fmt.Println("  [warning] real generation failed -> synthetic fallback:", err)
		base = synthBase()
	}
	// Segmentation & alignment: a high-contrast case among sample/ real AI matted strips (selected by scan)
	segMatte, segN := pickSampleStrip("sample/fire-mage/kick-south-east", 9)
	cenMatte, cenN := pickSampleStrip("sample/fire-mage/dash", 5)

	demoMatting(base)
	demoSegmentation(segMatte, segN)
	demoCentroid(cenMatte, cenN)
	demoPixelize(base)
	demoBodyExtent()
	demoOverlapRecovery()
	buildOverview()
	fmt.Println("Done. Check report-images/.")
}

// pickSampleStrip reads sample/<char>/<state>/_strip.png (a real AI matted strip).
// If absent, it falls back to matting a synthetic strip. (n = number of frames in the directory)
func pickSampleStrip(dir string, fallbackN int) (*image.NRGBA, int) {
	p := filepath.Join(dir, "_strip.png")
	if im, err := loadPNG(p); err == nil {
		n := countFrames(dir)
		if n < 2 {
			n = fallbackN
		}
		fmt.Printf("  using strip: %s (n=%d)\n", p, n)
		return im, n
	}
	fmt.Printf("  [warning] %s not found -> synthetic matting fallback\n", p)
	return sprite.RemoveBackground(synthStrip(fallbackN)), fallbackN
}

// ---- synthetic fallback input (used only when real generation fails) ----

func synthBase() *image.NRGBA {
	im := magentaCanvas(cell, cell)
	drawChar(im, cell/2, cell-28, 1.0, idlePose())
	shadeGradient(im)
	return jpegRoundTrip(im, 88)
}

func synthStrip(n int) *image.NRGBA {
	W := cell * n
	im := magentaCanvas(W, cell)
	for i := 0; i < n; i++ {
		p := idlePose()
		p.armLen = 70
		swing := float64((i%2)*2-1) * 55
		p.rArm, p.lArm = swing, -swing/2
		p.lLeg, p.rLeg = swing/4, -swing/4
		drawChar(im, cell*i+cell/2, cell-28, 1.0, p)
	}
	return jpegRoundTrip(im, 86)
}

// ensure sprite import used.
var _ = sprite.RemoveBackground
