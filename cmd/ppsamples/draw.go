package main

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Synthetic character/strip rendering + labels + compositing helpers.
// Without any AI calls, it draws a simple humanoid over a magenta background to
// demonstrate the before/after of the sprite pipeline (background removal,
// segmentation, alignment, pixelization).

var (
	keyMagenta = color.NRGBA{255, 0, 255, 255}
	colSkin    = color.NRGBA{242, 200, 158, 255}
	colShirt   = color.NRGBA{56, 122, 201, 255}
	colPants   = color.NRGBA{60, 64, 84, 255}
	colHair    = color.NRGBA{120, 72, 40, 255}
	colOutline = color.NRGBA{26, 28, 40, 255}
	colBG      = color.NRGBA{232, 234, 238, 255} // light gray for transparency demos
	colInk     = color.NRGBA{30, 32, 44, 255}
	colWith    = color.NRGBA{20, 140, 80, 255}
	colWithout = color.NRGBA{200, 60, 60, 255}
)

func newCanvas(w, h int, fill color.NRGBA) *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i+3 < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = fill.R, fill.G, fill.B, fill.A
	}
	return im
}

func setPx(im *image.NRGBA, x, y int, c color.NRGBA) {
	if x < 0 || y < 0 || x >= im.Rect.Dx() || y >= im.Rect.Dy() {
		return
	}
	i := im.PixOffset(x, y)
	im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, c.A
}

func fillRect(im *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			setPx(im, x, y, c)
		}
	}
}

func fillDisc(im *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				setPx(im, x, y, c)
			}
		}
	}
}

// thickLine draws (x0,y0)->(x1,y1) as a radius-r capsule (for rotatable limbs).
func thickLine(im *image.NRGBA, x0, y0, x1, y1, r int, c color.NRGBA) {
	steps := int(math.Hypot(float64(x1-x0), float64(y1-y0))) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		x := int(float64(x0) + t*float64(x1-x0) + 0.5)
		y := int(float64(y0) + t*float64(y1-y0) + 0.5)
		fillDisc(im, x, y, r, c)
	}
}

// pose holds a frame's limb angles (in degrees).
type pose struct {
	lArm, rArm, lLeg, rLeg float64
	armLen, legLen         int
}

// drawChar draws a humanoid with (footX, footY) as the foot anchor. sc is the size scale.
func drawChar(im *image.NRGBA, footX, footY int, sc float64, p pose) {
	S := func(v int) int { return int(float64(v) * sc) }
	hipY := footY - S(70)
	shoulderY := footY - S(120)
	headR := S(22)
	headCY := shoulderY - headR - S(4)

	deg := func(d float64) (float64, float64) { return math.Sin(d * math.Pi / 180), math.Cos(d * math.Pi / 180) }
	out := S(4) // outline thickness margin

	// legs (outline first, color on top)
	for _, leg := range []struct {
		ang  float64
		sign int
	}{{p.lLeg, -1}, {p.rLeg, 1}} {
		sn, cs := deg(leg.ang)
		hx := footX + leg.sign*S(10)
		ex := hx + int(sn*float64(S(p.legLen)))
		ey := hipY + int(cs*float64(S(p.legLen)))
		thickLine(im, hx, hipY, ex, ey, S(9)+out, colOutline)
		thickLine(im, hx, hipY, ex, ey, S(9), colPants)
	}
	// arms
	for _, arm := range []struct {
		ang  float64
		sign int
	}{{p.lArm, -1}, {p.rArm, 1}} {
		sn, cs := deg(arm.ang)
		sx := footX + arm.sign*S(16)
		ex := sx + int(sn*float64(S(p.armLen)))
		ey := shoulderY + int(cs*float64(S(p.armLen)))
		thickLine(im, sx, shoulderY, ex, ey, S(7)+out, colOutline)
		thickLine(im, sx, shoulderY, ex, ey, S(7), colSkin)
	}
	// torso
	fillRect(im, footX-S(20)-out, shoulderY-out, footX+S(20)+out, hipY+out, colOutline)
	fillRect(im, footX-S(20), shoulderY, footX+S(20), hipY, colShirt)
	// head + hair
	fillDisc(im, footX, headCY, headR+out, colOutline)
	fillDisc(im, footX, headCY, headR, colSkin)
	fillRect(im, footX-headR, headCY-headR, footX+headR+1, headCY-S(4), colHair)
}

func drawText(im *image.NRGBA, x, y int, s string, c color.NRGBA) {
	d := &font.Drawer{
		Dst:  im,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// checker draws a checkerboard background that makes transparent areas visible.
func checker(w, h, sz int) *image.NRGBA {
	im := newCanvas(w, h, color.NRGBA{255, 255, 255, 255})
	c2 := color.NRGBA{205, 208, 214, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x/sz+y/sz)%2 == 1 {
				setPx(im, x, y, c2)
			}
		}
	}
	return im
}
