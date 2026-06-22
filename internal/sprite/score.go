package sprite

import (
	"image"
	"math"
)

// ScoreResult holds the quality metrics of a frame set.
type ScoreResult struct {
	Identity float64 `json:"identity"` // average perceptual similarity between adjacent frames (0~1)
	Motion   float64 `json:"motion"`   // MotionPresence 0~1
	Contact  float64 `json:"contact"`  // ground-line/edge consistency 0~1
	Overall  float64 `json:"overall"`  // 0~1 overall score
}

// ScoreFrames computes the completeness score of a frame set.
func ScoreFrames(frames []*image.NRGBA) ScoreResult {
	r := ScoreResult{}
	if len(frames) < 2 {
		return r
	}
	r.Motion = MotionPresence(frames)
	r.Identity = pairwiseIdentity(frames)
	r.Contact = contactScore(frames)
	r.Overall = 0.5*r.Identity + 0.3*r.Motion + 0.2*r.Contact
	return r
}

// pairwiseIdentity normalizes the weighted color/alpha difference between adjacent frames to 0~1.
func pairwiseIdentity(frames []*image.NRGBA) float64 {
	var total float64
	pairs := 0
	for i := 1; i < len(frames); i++ {
		a, b := frames[i-1], frames[i]
		if a.Rect != b.Rect {
			continue
		}
		var diff float64
		var n int
		for p := 0; p+3 < len(a.Pix) && p+3 < len(b.Pix); p += 4 {
			// perceptually weighted color distance + alpha difference
			dr := float64(int(a.Pix[p]) - int(b.Pix[p]))
			dg := float64(int(a.Pix[p+1]) - int(b.Pix[p+1]))
			db := float64(int(a.Pix[p+2]) - int(b.Pix[p+2]))
			da := float64(int(a.Pix[p+3]) - int(b.Pix[p+3]))
			// perceptual RGB distance
			d := math.Sqrt(0.299*dr*dr + 0.587*dg*dg + 0.114*db*db)
			d += 0.5 * math.Abs(da)
			if a.Pix[p+3] > alphaThreshold || b.Pix[p+3] > alphaThreshold {
				diff += math.Min(d/(255.0*1.5), 1.0)
				n++
			}
		}
		if n > 0 {
			total += 1.0 - diff/float64(n)
			pairs++
		}
	}
	if pairs == 0 {
		return 0
	}
	return total / float64(pairs)
}

// contactScore measures the vertical consistency of the baseline/top contact.
// It gives a low score when the character's foot/head height changes greatly between frames.
func contactScore(frames []*image.NRGBA) float64 {
	type bounds struct {
		top, bottom, h int
		has            bool
	}
	bbs := make([]bounds, len(frames))
	for i, f := range frames {
		w, h := f.Rect.Dx(), f.Rect.Dy()
		top, bottom := -1, -1
		for y := 0; y < h; y++ {
			rowOpaque := false
			for x := 0; x < w; x++ {
				if f.Pix[f.PixOffset(x, y)+3] > alphaThreshold {
					rowOpaque = true
					break
				}
			}
			if rowOpaque {
				if top < 0 {
					top = y
				}
				bottom = y
			}
		}
		bbs[i] = bounds{top, bottom, h, top >= 0}
	}
	var n int
	meanBottom, meanTop := 0.0, 0.0
	maxH := 1
	for _, b := range bbs {
		if b.h > maxH {
			maxH = b.h
		}
		if b.has {
			meanBottom += float64(b.bottom)
			meanTop += float64(b.top)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	meanBottom /= float64(n)
	meanTop /= float64(n)
	var bottomVar, topVar float64
	for _, b := range bbs {
		if b.has {
			bottomVar += math.Abs(float64(b.bottom) - meanBottom)
			topVar += math.Abs(float64(b.top) - meanTop)
		}
	}
	bottomMAE := bottomVar / float64(n)
	topMAE := topVar / float64(n)
	// Tolerance relative to height: top (head) change within 28%, bottom (feet) within 10%.
	// In swimming/jumping the feet stay fixed while the upper body moves up and down, so
	// contact should be high when the bottom is stable.
	tolBottom := math.Max(float64(maxH)*0.10, 2.0)
	tolTop := math.Max(float64(maxH)*0.28, 2.0)
	bottomScore := 1.0 - math.Min(bottomMAE/tolBottom, 1.0)
	topScore := 1.0 - math.Min(topMAE/tolTop, 1.0)
	return 0.75*bottomScore + 0.25*topScore
}
