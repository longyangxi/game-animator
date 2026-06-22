package main

import (
	"image"
	"math"
)

// magentaResidue counts opaque pixels close to magenta (a background-removal residue metric).
func magentaResidue(im *image.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
		if r > 150 && b > 150 && g < 130 {
			n++
		}
	}
	return n
}

// haloPinkish counts opaque pixels at pinkish, magenta-tinted edges (the halo).
// Not pure magenta, but pixels where R and B are clearly higher than G (the key color has bled).
func haloPinkish(im *image.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
		if r-g > 40 && b-g > 40 && !(r > 200 && b > 200 && g < 80) {
			n++
		}
	}
	return n
}

// distinctColors counts the number of distinct RGB values among opaque pixels (a pixelization metric).
func distinctColors(im *image.NRGBA) int {
	set := map[uint32]struct{}{}
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		k := uint32(im.Pix[i])<<16 | uint32(im.Pix[i+1])<<8 | uint32(im.Pix[i+2])
		set[k] = struct{}{}
	}
	return len(set)
}

// edgeContent counts opaque pixels touching the left/right edges of the cell (2px).
// It runs high when equal splitting has cut through a pose.
func edgeContent(im *image.NRGBA) int {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	n := 0
	for y := 0; y < h; y++ {
		for _, x := range []int{0, 1, w - 2, w - 1} {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				n++
			}
		}
	}
	return n
}

// torsoCenterX returns the horizontal center of shirt-colored (blue) pixels (the torso position).
func torsoCenterX(im *image.NRGBA) float64 {
	var sx, n float64
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			i := im.PixOffset(x, y)
			if im.Pix[i+3] <= aThresh {
				continue
			}
			r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
			if b > 140 && b-r > 40 && g < 180 { // approximate shirt blue
				sx += float64(x)
				n++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return sx / n
}

// stdDev returns the standard deviation of the values (a torso-jitter metric).
func stdDev(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	var m float64
	for _, v := range vs {
		m += v
	}
	m /= float64(len(vs))
	var s float64
	for _, v := range vs {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(vs)))
}

// colAlpha returns the per-column opaque pixel count (the alpha mass profile).
func colAlpha(im *image.NRGBA) []float64 {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	p := make([]float64, w)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				p[x]++
			}
		}
	}
	return p
}

// crossingLines counts how many of the equal N-division lines (k*W/n) cross the
// character (content). Projection-based splitting cuts at empty gutters so it's
// 0, but equal-split cuts across the pose.
func crossingLines(im *image.NRGBA, n int) (cross int, lineMass []float64) {
	p := colAlpha(im)
	w := len(p)
	mx := 0.0
	for _, v := range p {
		if v > mx {
			mx = v
		}
	}
	thresh := 0.06 * mx // at least 6% of the max column mass counts as "over content"
	for k := 1; k < n; k++ {
		x := k * w / n
		m := p[x]
		lineMass = append(lineMass, m)
		if m > thresh {
			cross++
		}
	}
	return
}

// contentPixels counts opaque pixels.
func contentPixels(im *image.NRGBA) int {
	n := 0
	for i := 3; i < len(im.Pix); i += 4 {
		if im.Pix[i] > aThresh {
			n++
		}
	}
	return n
}

// frameBalance returns the min/max content pixels across frames and the
// imbalance ratio (relative to the min). When equal splitting produces empty
// cells or half poses, min drops sharply and the ratio grows.
func frameBalance(frames []*image.NRGBA) (min, max int, ratio float64) {
	min = 1 << 30
	for _, f := range frames {
		c := contentPixels(f)
		if c < min {
			min = c
		}
		if c > max {
			max = c
		}
	}
	if min <= 0 {
		min = 0
		ratio = 999
	} else {
		ratio = float64(max) / float64(min)
	}
	return
}

func torsoSpread(frames []*image.NRGBA) float64 {
	var xs []float64
	for _, f := range frames {
		xs = append(xs, torsoCenterX(f))
	}
	return stdDev(xs)
}

// contentCentroidX returns the horizontal centroid of opaque pixels in the cell (color-agnostic).
func contentCentroidX(im *image.NRGBA) float64 {
	var sx, n float64
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				sx += float64(x)
				n++
			}
		}
	}
	if n == 0 {
		return float64(im.Rect.Dx()) / 2
	}
	return sx / n
}

// centroidSpread is the standard deviation of the per-frame content centroid X (anchor jitter, color-agnostic).
func centroidSpread(frames []*image.NRGBA) float64 {
	var xs []float64
	for _, f := range frames {
		xs = append(xs, contentCentroidX(f))
	}
	return stdDev(xs)
}
