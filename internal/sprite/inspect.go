package sprite

import (
	"fmt"
	"image"
	"sort"
)

// Frame quality inspection parameters
const (
	inspectEdgeMargin      = 2    // edge inspection width (px)
	inspectEdgeMax         = 24   // edge pixels beyond this count risk being cut off
	inspectKeyDist         = 70.0 // within this distance from the background key, a residual chroma candidate
	inspectKeyMax          = 120  // allowed limit of residual chroma pixels
	inspectSmallRatio      = 0.35 // below this ratio of the median, an abnormally small frame
	inspectLargeRatio      = 2.75 // above this ratio of the median, an abnormally large frame
	inspectMinContentAbs   = 400  // minimum content pixels per frame (absolute)
	inspectContentMinAlpha = 0.25 // opaque below this ratio of the whole frame allows aggressive space saving
	driftWarnSim           = 0.65 // below this color-composition similarity, warn about character drift
	driftErrorSim          = 0.45 // below this, severe drift → regeneration required
	baseWarnSim            = 0.60 // warning threshold for average similarity against the base character
	baseErrorSim           = 0.40 // below this, every frame is a different character from the base → regenerate
)

// keyTinted reports whether a pixel carries the background key tint (residual chroma/halo).
// On the key's strong channels (>192) the pixel is also high (>150), and on the key's weak
// channels (<64) the pixel is also low (<110), it is treated as carrying the key tint. This
// avoids false positives on the character's reds/blues, etc.
func keyTinted(r, g, b uint8, key [3]uint8) bool {
	px := [3]uint8{r, g, b}
	for c := 0; c < 3; c++ {
		if key[c] > 192 {
			if px[c] <= 150 {
				return false
			}
		} else if key[c] < 64 {
			if px[c] >= 110 {
				return false
			}
		}
	}
	return true
}

// FrameReport holds the quality measurements for a single frame.
type FrameReport struct {
	Index         int     `json:"index"`
	ContentPixels int     `json:"contentPixels"`
	EdgePixels    int     `json:"edgePixels"`
	KeyResidue    int     `json:"keyResidue"`
	PaletteSim    float64 `json:"paletteSim"` // color-composition similarity to other frames (0~1)
}

// InspectResult is the result of a frame quality inspection.
type InspectResult struct {
	Reports    []FrameReport
	Errors     []string    // serious problems requiring regeneration (English, for user display)
	Warnings   []string    // informational warnings (English, for user display)
	RetryHints []string    // corrective instructions to inject into the regeneration prompt (English)
	Score      ScoreResult `json:"score"`
}

// Ok returns true when there are no serious quality problems.
func (r InspectResult) Ok() bool { return len(r.Errors) == 0 }

// InspectFrames inspects the quality of the extracted frames.
// key is the chroma background color used for generation (for residual chroma detection).
// If base is not nil, an identity check against the base character is also performed.
// The inter-frame (leave-one-out) check misses the case where all frames drift together,
// but the check against the base catches it.
func InspectFrames(frames []*image.NRGBA, key [3]uint8, base *image.NRGBA) InspectResult {
	var res InspectResult
	if len(frames) == 0 {
		return res
	}

	hintSet := map[string]bool{}
	addHint := func(h string) {
		if !hintSet[h] {
			hintSet[h] = true
			res.RetryHints = append(res.RetryHints, h)
		}
	}

	areas := make([]int, 0, len(frames))
	var opaqueTotal int
	for _, f := range frames {
		w, h := f.Rect.Dx(), f.Rect.Dy()
		opaqueTotal += w * h
	}
	contentAlphaCutoff := int(float64(opaqueTotal/len(frames)) * inspectContentMinAlpha)
	for i, f := range frames {
		rep := FrameReport{Index: i}
		w, h := f.Rect.Dx(), f.Rect.Dy()

		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				pi := f.PixOffset(x, y)
				if f.Pix[pi+3] <= alphaThreshold {
					continue
				}
				rep.ContentPixels++
				if x < inspectEdgeMargin || x >= w-inspectEdgeMargin ||
					y < inspectEdgeMargin || y >= h-inspectEdgeMargin {
					rep.EdgePixels++
				}
				pr, pg, pb := f.Pix[pi], f.Pix[pi+1], f.Pix[pi+2]
				if colorDist(pr, pg, pb, key) <= inspectKeyDist && keyTinted(pr, pg, pb, key) {
					rep.KeyResidue++
				}
			}
		}

		minContent := inspectMinContentAbs
		if rel := w * h / 100; rel > minContent {
			minContent = rel
		}
		if rep.ContentPixels < minContent {
			res.Errors = append(res.Errors,
				fmt.Sprintf("Frame %d is empty or too faint (%d pixels)", i+1, rep.ContentPixels))
			addHint("Every column must hold one complete, fully drawn full-body character. Leave no column empty or faint.")
		}
		if rep.EdgePixels > inspectEdgeMax {
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("Frame %d touches the edge and may be cut off (%d pixels)", i+1, rep.EdgePixels))
			addHint("Keep every pose fully inside its column with clear padding on all sides; no body part may touch or cross a column edge.")
		}
		if rep.KeyResidue > inspectKeyMax {
			res.Errors = append(res.Errors,
				fmt.Sprintf("Frame %d still contains background-color residue (%d pixels)", i+1, rep.KeyResidue))
			addHint("The character must not contain magenta or magenta-adjacent colors anywhere (clothes, effects, highlights). Keep the background a perfectly flat pure magenta #FF00FF and keep all character colors far from magenta.")
		}

		if contentAlphaCutoff > 0 && rep.ContentPixels < contentAlphaCutoff {
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("Frame %d has significantly less content than the other frames (deviation: %d%%)", i+1, int((1-float64(rep.ContentPixels)/float64(contentAlphaCutoff))*100)))
			addHint("Draw the character at a consistent size across the strip; no pose may be much smaller or partially erased.")
		}

		res.Reports = append(res.Reports, rep)
		areas = append(areas, rep.ContentPixels)
	}

	// Size consistency: detect frames that are too small/large relative to the median
	if len(areas) >= 3 {
		sorted := append([]int(nil), areas...)
		sort.Ints(sorted)
		median := sorted[len(sorted)/2]
		if median > 0 {
			for i, a := range areas {
				ratio := float64(a) / float64(median)
				if ratio < inspectSmallRatio {
					res.Warnings = append(res.Warnings,
						fmt.Sprintf("Frame %d is abnormally too small compared to the other frames", i+1))
					addHint("Draw the character at the same scale in every frame; no pose may be much smaller or larger than the others.")
				} else if ratio > inspectLargeRatio {
					res.Warnings = append(res.Warnings,
						fmt.Sprintf("Frame %d is abnormally large compared to the other frames (poses may have merged)", i+1))
					addHint("Each pose must be completely separate with clear magenta gaps between poses; poses must never touch, overlap, or merge.")
				}
			}
		}
	}

	// Character drift detection: if the pose-independent color composition (histogram)
	// differs greatly between frames, the AI is judged to have redrawn the character's
	// identity (hair color, outfit, etc.)
	if len(frames) >= 2 {
		hists := make([][histBins]float64, len(frames))
		for i, f := range frames {
			hists[i] = colorHistogram(f)
		}
		for i := range frames {
			// leave-one-out: compare against the average of the remaining frames
			var avg [histBins]float64
			for j := range frames {
				if j == i {
					continue
				}
				for k := 0; k < histBins; k++ {
					avg[k] += hists[j][k]
				}
			}
			n := float64(len(frames) - 1)
			sim := 0.0
			for k := 0; k < histBins; k++ {
				sim += minf(hists[i][k], avg[k]/n)
			}
			res.Reports[i].PaletteSim = sim
			if sim < driftErrorSim {
				res.Errors = append(res.Errors,
					fmt.Sprintf("Frame %d's color composition differs greatly from the other frames (suspected character change, similarity %.0f%%)", i+1, sim*100))
				addHint("CRITICAL: keep the exact same character identity in every frame — identical hair color, skin tone, outfit colors and proportions. Only the pose may change between frames.")
			} else if sim < driftWarnSim {
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("Frame %d's color composition differs somewhat from the other frames (similarity %.0f%%)", i+1, sim*100))
				addHint("Keep the character's colors and details consistent across all frames; do not change hair, skin or outfit colors between poses.")
			}
		}
	}

	// Identity check against the base character: if the frames' average color composition
	// differs greatly from the base, the whole set was drawn as a different character
	// (a batch drift that per-frame checks cannot catch)
	if base != nil && len(frames) > 0 && hasTransparency(base) {
		baseHist := colorHistogram(base)
		totalSim := 0.0
		for _, f := range frames {
			h := colorHistogram(f)
			sim := 0.0
			for k := 0; k < histBins; k++ {
				sim += minf(h[k], baseHist[k])
			}
			totalSim += sim
		}
		avg := totalSim / float64(len(frames))
		if avg < baseErrorSim {
			res.Errors = append(res.Errors,
				fmt.Sprintf("The generated frames differ greatly from the base character (similarity %.0f%%)", avg*100))
			res.RetryHints = append(res.RetryHints,
				"CRITICAL: the previous attempt drew a different-looking character. Copy the attached reference image's identity exactly — identical hair color, skin tone, outfit colors, proportions and accessories in every frame.")
		} else if avg < baseWarnSim {
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("The generated frames' color composition differs somewhat from the base character (similarity %.0f%%)", avg*100))
		}
	}

	// Also compute the score at inspection time and include it in InspectResult
	res.Score = ScoreFrames(frames)

	return res
}

// MotionPresence returns the average change rate between adjacent frames (0~1).
// For two frames of the same size, it averages the normalized RGBA difference over pixels
// that are opaque in either frame. A value near 0 means an effectively static image (no real
// motion); excluding intentional stillness like idle/sleep, it signals an "animation that does
// not move" defect. It measures the opposite axis (too similar) from InspectFrames' drift
// check (frames too different).
func MotionPresence(frames []*image.NRGBA) float64 {
	if len(frames) < 2 {
		return 0
	}
	var total float64
	pairs := 0
	for i := 1; i < len(frames); i++ {
		a, b := frames[i-1], frames[i]
		if a.Rect != b.Rect {
			continue
		}
		var diffSum float64
		var count int
		for p := 0; p+3 < len(a.Pix) && p+3 < len(b.Pix); p += 4 {
			aa, ba := a.Pix[p+3], b.Pix[p+3]
			if aa <= alphaThreshold && ba <= alphaThreshold {
				continue
			}
			d := absDiff(a.Pix[p], b.Pix[p]) + absDiff(a.Pix[p+1], b.Pix[p+1]) +
				absDiff(a.Pix[p+2], b.Pix[p+2]) + absDiff(aa, ba)
			diffSum += float64(d) / (255.0 * 4.0)
			count++
		}
		if count > 0 {
			total += diffSum / float64(count)
			pairs++
		}
	}
	if pairs == 0 {
		return 0
	}
	return total / float64(pairs)
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// hasTransparency checks whether the image has a meaningful transparent region.
// Opaque-background images (photos, etc.) are excluded from inspection because the background
// color pollutes the histogram and causes false positives in the check against the base.
func hasTransparency(img *image.NRGBA) bool {
	total, transparent := 0, 0
	for i := 3; i < len(img.Pix); i += 4 {
		total++
		if img.Pix[i] <= alphaThreshold {
			transparent++
		}
	}
	return total > 0 && float64(transparent)/float64(total) >= 0.05
}

const histBins = 64 // 4×4×4 RGB quantization bins

// colorHistogram computes a normalized coarse RGB histogram of the opaque pixels.
// It exploits the fact that the same character has a similar distribution regardless of pose.
func colorHistogram(f *image.NRGBA) [histBins]float64 {
	var hist [histBins]float64
	total := 0
	for i := 0; i+3 < len(f.Pix); i += 4 {
		if f.Pix[i+3] <= alphaThreshold {
			continue
		}
		bin := int(f.Pix[i]>>6)<<4 | int(f.Pix[i+1]>>6)<<2 | int(f.Pix[i+2]>>6)
		hist[bin]++
		total++
	}
	if total > 0 {
		for k := range hist {
			hist[k] /= float64(total)
		}
	}
	return hist
}
