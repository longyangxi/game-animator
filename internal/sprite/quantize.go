package sprite

import (
	"image"
	"sort"
)

// rgb is an opaque color used for quantization.
type rgb struct{ r, g, b uint8 }

// collectOpaque collects opaque pixel colors from the frames (with downsampling).
func collectOpaque(frames []*image.NRGBA, maxSamples int) []rgb {
	total := 0
	for _, f := range frames {
		total += len(f.Pix) / 4
	}
	step := 1
	if maxSamples > 0 && total > maxSamples {
		step = total/maxSamples + 1
	}
	var out []rgb
	idx := 0
	for _, f := range frames {
		for i := 0; i+3 < len(f.Pix); i += 4 {
			if f.Pix[i+3] <= alphaThreshold {
				continue
			}
			if idx%step == 0 {
				out = append(out, rgb{f.Pix[i], f.Pix[i+1], f.Pix[i+2]})
			}
			idx++
		}
	}
	return out
}

// BuildSharedPalette extracts a shared palette across all frames (median-cut).
// Forcing the same palette on every animation frame greatly improves color consistency between frames.
func BuildSharedPalette(frames []*image.NRGBA, maxColors int) []rgb {
	if maxColors < 2 {
		maxColors = 2
	}
	samples := collectOpaque(frames, 1<<16)
	if len(samples) == 0 {
		return nil
	}
	buckets := [][]rgb{samples}
	for len(buckets) < maxColors {
		// pick the bucket with the largest variance range
		bestIdx, bestRange := -1, 0
		bestCh := 0
		for bi, b := range buckets {
			if len(b) < 2 {
				continue
			}
			minC := [3]int{255, 255, 255}
			maxC := [3]int{0, 0, 0}
			for _, c := range b {
				ch := [3]int{int(c.r), int(c.g), int(c.b)}
				for k := 0; k < 3; k++ {
					if ch[k] < minC[k] {
						minC[k] = ch[k]
					}
					if ch[k] > maxC[k] {
						maxC[k] = ch[k]
					}
				}
			}
			for k := 0; k < 3; k++ {
				if r := maxC[k] - minC[k]; r > bestRange {
					bestIdx, bestRange, bestCh = bi, r, k
				}
			}
		}
		if bestIdx < 0 || bestRange == 0 {
			break // cannot split any further
		}
		b := buckets[bestIdx]
		sort.Slice(b, func(i, j int) bool {
			switch bestCh {
			case 0:
				return b[i].r < b[j].r
			case 1:
				return b[i].g < b[j].g
			default:
				return b[i].b < b[j].b
			}
		})
		mid := len(b) / 2
		buckets[bestIdx] = b[:mid]
		buckets = append(buckets, b[mid:])
	}

	type entry struct {
		c rgb
		n int
	}
	entries := make([]entry, 0, len(buckets))
	for _, b := range buckets {
		if len(b) == 0 {
			continue
		}
		var sr, sg, sb int
		for _, c := range b {
			sr += int(c.r)
			sg += int(c.g)
			sb += int(c.b)
		}
		n := len(b)
		entries = append(entries, entry{rgb{uint8(sr / n), uint8(sg / n), uint8(sb / n)}, n})
	}

	// merge nearby colors: converge slight inter-frame color drift (~8 per channel) into a single color
	const mergeThresh = 600 // by colorDist2 ≈ 8 per channel
	sort.Slice(entries, func(i, j int) bool { return entries[i].n > entries[j].n })
	merged := entries[:0]
	for _, e := range entries {
		absorbed := false
		for mi := range merged {
			if colorDist2(e.c, merged[mi].c) < mergeThresh {
				// absorb by weighted average
				tot := merged[mi].n + e.n
				merged[mi].c = rgb{
					uint8((int(merged[mi].c.r)*merged[mi].n + int(e.c.r)*e.n) / tot),
					uint8((int(merged[mi].c.g)*merged[mi].n + int(e.c.g)*e.n) / tot),
					uint8((int(merged[mi].c.b)*merged[mi].n + int(e.c.b)*e.n) / tot),
				}
				merged[mi].n = tot
				absorbed = true
				break
			}
		}
		if !absorbed {
			merged = append(merged, e)
		}
	}
	// if empty buckets / insufficient palette leave fewer than 2 colors, add at least black and white grays
	if len(merged) < 2 {
		if len(merged) == 0 {
			merged = append(merged, entry{rgb{0, 0, 0}, 1}, entry{rgb{255, 255, 255}, 1})
		} else {
			merged = append(merged, entry{rgb{255, 255, 255}, 1})
		}
	}
	palette := make([]rgb, len(merged))
	for i, e := range merged {
		palette[i] = e.c
	}
	return palette
}

func colorDist2(a, b rgb) int {
	dr, dg, db := int(a.r)-int(b.r), int(a.g)-int(b.g), int(a.b)-int(b.b)
	// perceptual weighting (higher sensitivity to green)
	return 2*dr*dr + 4*dg*dg + 3*db*db
}

func nearestColor(c rgb, palette []rgb, cache map[rgb]rgb) rgb {
	if hit, ok := cache[c]; ok {
		return hit
	}
	best, bestD := palette[0], 1<<62
	for _, p := range palette {
		if d := colorDist2(c, p); d < bestD {
			best, bestD = p, d
		}
	}
	cache[c] = best
	return best
}

// ApplyPalette replaces every opaque pixel in the image with the nearest palette color and
// binarizes the alpha to 0/255 (pixel art does not use partial transparency).
func ApplyPalette(img *image.NRGBA, palette []rgb) {
	if len(palette) == 0 {
		return
	}
	cache := make(map[rgb]rgb, 512)
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i+3] < 128 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 0, 0, 0
			continue
		}
		img.Pix[i+3] = 255
		c := nearestColor(rgb{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}, palette, cache)
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = c.r, c.g, c.b
	}
}
