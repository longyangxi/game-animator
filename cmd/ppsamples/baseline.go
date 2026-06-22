package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"

	xdraw "golang.org/x/image/draw"
)

const aThresh = 10 // same empty-pixel threshold as the sprite package

// jpegRoundTrip encodes/decodes a synthetic image through JPEG to introduce
// 4:2:0 chroma subsampling artifacts (block noise, edge color bleed) that mimic
// real AI output.
func jpegRoundTrip(im *image.NRGBA, q int) *image.NRGBA {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, im, &jpeg.Options{Quality: q})
	dec, _ := jpeg.Decode(&buf)
	out := image.NewNRGBA(im.Rect)
	xdraw.Copy(out, image.Point{}, dec, dec.Bounds(), xdraw.Src, nil)
	return out
}

// naiveMatte is the "no technique" background removal: it cuts using a single
// pure RGB distance threshold. No soft alpha, despill, flood fill, or
// morphology -> halos/residue remain.
func naiveMatte(src image.Image, key color.NRGBA, tol float64) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Copy(out, image.Point{}, src, b, xdraw.Src, nil)
	for i := 0; i+3 < len(out.Pix); i += 4 {
		dr := float64(out.Pix[i]) - float64(key.R)
		dg := float64(out.Pix[i+1]) - float64(key.G)
		db := float64(out.Pix[i+2]) - float64(key.B)
		if dr*dr+dg*dg+db*db <= tol*tol {
			out.Pix[i+3] = 0 // background -> transparent (RGB left intact, so halo remains)
		}
	}
	return out
}

// equalSplitExtract is the "no technique" frame extraction: it splits the strip
// into n equal parts and places each part's content into a cell by bbox center.
// (No projection/DP, no centroid.)
func equalSplitExtract(strip *image.NRGBA, n, cellW, cellH, margin int) []*image.NRGBA {
	w, h := strip.Rect.Dx(), strip.Rect.Dy()
	var frames []*image.NRGBA
	for k := 0; k < n; k++ {
		x0 := w * k / n
		x1 := w * (k + 1) / n
		frames = append(frames, placeBBoxCenter(strip, x0, x1, h, cellW, cellH, margin))
	}
	return frames
}

// placeBBoxCenter centers the content of the [x0,x1) span in the cell by bbox center.
func placeBBoxCenter(strip *image.NRGBA, x0, x1, h, cellW, cellH, margin int) *image.NRGBA {
	minX, minY, maxX, maxY := x1, h, x0-1, -1
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
	cell := newCanvas(cellW, cellH, color.NRGBA{0, 0, 0, 0})
	if maxX < minX {
		return cell
	}
	gw, gh := maxX-minX+1, maxY-minY+1
	avail := cellH - margin*2
	scale := float64(avail) / float64(gh)
	if scale > 1 {
		scale = 1
	}
	sw, sh := int(float64(gw)*scale+0.5), int(float64(gh)*scale+0.5)
	src := image.NewNRGBA(image.Rect(0, 0, gw, gh))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			si := strip.PixOffset(x, y)
			if strip.Pix[si+3] > aThresh {
				di := src.PixOffset(x-minX, y-minY)
				copy(src.Pix[di:di+4], strip.Pix[si:si+4])
			}
		}
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	xdraw.CatmullRom.Scale(scaled, scaled.Rect, src, src.Rect, xdraw.Over, nil)
	left := (cellW - sw) / 2 // bbox center at the cell center (<- the crux of "no technique")
	top := cellH - margin - sh
	xdraw.Copy(cell, image.Point{X: left, Y: top}, scaled, scaled.Rect, xdraw.Over, nil)
	return cell
}

// overOn builds a new image by alpha-compositing src over bg (for demoing transparent results).
func overOn(bg, src *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(bg.Rect)
	copy(out.Pix, bg.Pix)
	xdraw.Copy(out, image.Point{}, src, src.Rect, xdraw.Over, nil)
	return out
}

// paste pastes src onto dst at (x,y).
func paste(dst, src *image.NRGBA, x, y int) {
	xdraw.Copy(dst, image.Point{X: x, Y: y}, src, src.Rect, xdraw.Over, nil)
}

// resizeW resamples to width targetW, preserving aspect ratio.
func resizeW(im *image.NRGBA, targetW int) *image.NRGBA {
	if im.Rect.Dx() == targetW {
		return im
	}
	h := im.Rect.Dy() * targetW / im.Rect.Dx()
	if h < 1 {
		h = 1
	}
	out := image.NewNRGBA(image.Rect(0, 0, targetW, h))
	xdraw.CatmullRom.Scale(out, out.Rect, im, im.Rect, xdraw.Over, nil)
	return out
}
