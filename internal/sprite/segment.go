package sprite

import "image"

// Strip → frame segmentation: uses a vertical alpha projection profile rather than connected
// components (flood-fill). It counts the natural number of poses from the gutters (valleys) of
// per-column alpha mass, and when poses touch and there is no gutter, it uses dynamic
// programming (DP) to force expected-1 cuts that "split the least content." This is the
// projection-profile + optimal-cut technique used in OCR line/word segmentation.

// colSpan is a column interval [start, end) in the strip coordinate system.
type colSpan struct{ start, end int }

// projectAlpha computes the per-column alpha mass P[x] = Σ_y α(x,y).
func projectAlpha(img *image.NRGBA) []float64 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	p := make([]float64, w)
	for x := 0; x < w; x++ {
		var sum float64
		for y := 0; y < h; y++ {
			sum += float64(img.Pix[img.PixOffset(x, y)+3])
		}
		p[x] = sum
	}
	return p
}

// smoothProfile smooths the profile with a box moving average (suppresses compression noise/thin gaps).
func smoothProfile(p []float64, win int) []float64 {
	if win < 1 || len(p) == 0 {
		return p
	}
	out := make([]float64, len(p))
	half := win / 2
	for i := range p {
		var sum float64
		var n int
		for j := i - half; j <= i+half; j++ {
			if j >= 0 && j < len(p) {
				sum += p[j]
				n++
			}
		}
		out[i] = sum / float64(n)
	}
	return out
}

func maxOf(p []float64) float64 {
	m := 0.0
	for _, v := range p {
		if v > m {
			m = v
		}
	}
	return m
}

// contentRuns finds the contiguous spans (poses) where P exceeds eps.
// Spans narrower than minW or with a peak below peakMin are treated as speckle and discarded.
func contentRuns(p []float64, eps, peakMin float64, minW int) []colSpan {
	var runs []colSpan
	i := 0
	n := len(p)
	for i < n {
		if p[i] <= eps {
			i++
			continue
		}
		j := i
		peak := 0.0
		for j < n && p[j] > eps {
			if p[j] > peak {
				peak = p[j]
			}
			j++
		}
		if j-i >= minW && peak >= peakMin {
			runs = append(runs, colSpan{i, j})
		}
		i = j
	}
	return runs
}

// runMass is the sum of alpha mass within the span.
func runMass(p []float64, s colSpan) float64 {
	var m float64
	for x := s.start; x < s.end && x < len(p); x++ {
		m += p[x]
	}
	return m
}

// dropMinorRuns removes runs below frac of the maximum run mass (distant residue/speckle).
// It serves the same purpose as the old connected-component "seed threshold relative to max blob area" guard.
func dropMinorRuns(p []float64, runs []colSpan, frac float64) []colSpan {
	if len(runs) <= 1 {
		return runs
	}
	maxM := 0.0
	for _, r := range runs {
		if m := runMass(p, r); m > maxM {
			maxM = m
		}
	}
	thr := maxM * frac
	var out []colSpan
	for _, r := range runs {
		if runMass(p, r) >= thr {
			out = append(out, r)
		}
	}
	return out
}

// dpNCut finds the n-1 cut columns that divide [x0,x1) into exactly n segments.
// cost = Σ P[cut] (cutting where mass is low is cheap) + width regularization (penalty for deviating from the ideal width).
// Used to force touching poses into expected segments.
func dpNCut(p []float64, x0, x1, n int) []int {
	if n <= 1 || x1-x0 < n {
		return nil
	}
	width := x1 - x0
	ideal := float64(width) / float64(n)
	minW := int(ideal * 0.45)
	if minW < 2 {
		minW = 2
	}
	const lambda = 0.0015 // width regularization weight (relative to mass cost)

	cuts := n - 1
	type cell struct {
		cost float64
		prev int
	}
	dp := make([][]cell, cuts+1)
	for k := range dp {
		dp[k] = make([]cell, x1+1)
		for x := range dp[k] {
			dp[k][x].cost = 1e18
			dp[k][x].prev = -1
		}
	}
	dp[0][x0].cost = 0 // virtual start boundary
	for k := 1; k <= cuts; k++ {
		lo := x0 + (k-1)*minW
		for x := x0 + k*minW; x <= x1-(cuts-k+1)*minW; x++ {
			best := 1e18
			bestPrev := -1
			for xp := lo; xp <= x-minW; xp++ {
				if dp[k-1][xp].cost >= 1e17 {
					continue
				}
				d := float64(x-xp) - ideal
				c := dp[k-1][xp].cost + p[x] + lambda*d*d
				if c < best {
					best, bestPrev = c, xp
				}
			}
			dp[k][x] = cell{cost: best, prev: bestPrev}
		}
	}
	bestEnd, bestCost := -1, 1e18
	for x := x0 + cuts*minW; x <= x1-minW; x++ {
		d := float64(x1-x) - ideal
		c := dp[cuts][x].cost + lambda*d*d
		if c < bestCost {
			bestCost, bestEnd = c, x
		}
	}
	if bestEnd < 0 {
		return nil
	}
	out := make([]int, cuts)
	x := bestEnd
	for k := cuts; k >= 1; k-- {
		out[k-1] = x
		x = dp[k][x].prev
		if x < 0 {
			return nil
		}
	}
	return out
}

// segmentStrip divides the strip into expected column segments and also returns the number of
// natural poses detected. If the natural pose count equals expected, it cuts cleanly at the
// gutter centers; otherwise it force-splits into expected segments via DP.
func segmentStrip(img *image.NRGBA, expected int) (segs []colSpan, natural int) {
	w := img.Rect.Dx()
	if w == 0 || expected < 1 {
		return nil, 0
	}
	raw := projectAlpha(img)
	win := w / 220
	if win < 3 {
		win = 3
	}
	p := smoothProfile(raw, win)
	mx := maxOf(p)
	if mx <= 0 {
		return nil, 0
	}
	eps := 0.045 * mx
	peakMin := 0.18 * mx
	minRun := w / 100
	if minRun < 4 {
		minRun = 4
	}
	runs := contentRuns(p, eps, peakMin, minRun)
	runs = dropMinorRuns(p, runs, 0.20)
	if len(runs) == 0 {
		return nil, 0
	}

	// Estimate the pose count per run from the number of "torso peaks (prominence peaks),"
	// capped by the run width. Peaks decide "where to cut," width decides "into how many":
	// even when a single pose makes two peaks (torso + extended leg, as in a kick), if the run
	// width is a single pose width (median) it is grouped as 1 to prevent over-splitting. Only
	// runs widened by touching are split accordingly.
	med := medianRunWidth(runs)
	widthTotal := 0.0
	for _, r := range runs {
		widthTotal += float64(r.end - r.start)
	}
	for _, r := range runs {
		nPeaks := len(posePeaks(p, r.start, r.end))
		if len(runs) > 1 && med > 0 {
			maxByWidth := int(float64(r.end-r.start)/med + 0.5)
			if maxByWidth < 1 {
				maxByWidth = 1
			}
			if nPeaks > maxByWidth {
				nPeaks = maxByWidth
			}
		}
		// When there is almost no gap between poses (overlapping) and only one peak appears,
		// but the run width is at least 1.5× the average pose width, force a suspicion of 2.
		if nPeaks == 1 && len(runs) > 1 && med > 0 {
			if float64(r.end-r.start) > med*1.45 {
				nPeaks = 2
			}
		}
		if nPeaks <= 1 {
			segs = append(segs, r)
		} else {
			segs = append(segs, splitRange(p, r.start, r.end, nPeaks)...)
		}
	}

	// Forced recovery: if the detected count differs from expected and the total content width
	// can accommodate the minimum width for the expected count, split the whole strip into
	// expected segments evenly/by DP. This defends against the AI drawing poses fully joined
	// with no magenta gutter.
	if len(segs) != expected && widthTotal/float64(expected) >= 16 && w/expected >= 16 {
		segs = splitRange(p, 0, w, expected)
	}

	// Emitted frame count = estimated pose count (reported honestly without forcing expected via DP).
	return segs, len(segs)
}

// medianRunWidth is the median of the run widths (an estimate of the typical single-pose width).
func medianRunWidth(runs []colSpan) float64 {
	if len(runs) == 0 {
		return 0
	}
	ws := make([]int, len(runs))
	for i, r := range runs {
		ws[i] = r.end - r.start
	}
	for i := 1; i < len(ws); i++ {
		for j := i; j > 0 && ws[j-1] > ws[j]; j-- {
			ws[j-1], ws[j] = ws[j], ws[j-1]
		}
	}
	return float64(ws[len(ws)/2])
}

// posePeaks finds the strong peak (=pose) columns in the [s,e) span by prominence. A peak
// candidate is a local maximum at least 45% of the run maximum, and the valley between it and
// a higher peak must be deep enough (must dip below 62% of its own height) for it to count as
// a separate pose.
func posePeaks(p []float64, s, e int) []int {
	if e-s < 3 {
		return []int{(s + e) / 2}
	}
	runMax := 0.0
	for x := s; x < e; x++ {
		if p[x] > runMax {
			runMax = p[x]
		}
	}
	if runMax <= 0 {
		return []int{(s + e) / 2}
	}
	cand := []int{}
	for x := s + 1; x < e-1; x++ {
		if p[x] >= p[x-1] && p[x] > p[x+1] && p[x] >= 0.45*runMax {
			cand = append(cand, x)
		}
	}
	if len(cand) == 0 {
		return []int{(s + e) / 2}
	}
	keep := []int{}
	for _, m := range cand {
		prominent := true
		for _, k := range cand {
			if k == m || p[k] < p[m] {
				continue // only check valley depth against peaks higher than itself
			}
			lo, hi := m, k
			if lo > hi {
				lo, hi = hi, lo
			}
			vmin := p[lo]
			for x := lo; x <= hi; x++ {
				if p[x] < vmin {
					vmin = p[x]
				}
			}
			if vmin > 0.62*p[m] { // shallow valley between → part of the same pose
				prominent = false
				break
			}
		}
		if prominent {
			keep = append(keep, m)
		}
	}
	if len(keep) == 0 {
		return []int{cand[0]}
	}
	return keep
}

// splitRange divides the [s,e) span into n segments by DP minimum cut (even split on failure).
func splitRange(p []float64, s, e, n int) []colSpan {
	if n <= 1 || e-s < n {
		return []colSpan{{s, e}}
	}
	var out []colSpan
	if cuts := dpNCut(p, s, e, n); len(cuts) == n-1 {
		prev := s
		for _, c := range cuts {
			out = append(out, colSpan{prev, c})
			prev = c
		}
		out = append(out, colSpan{prev, e})
		return out
	}
	for i := 0; i < n; i++ {
		out = append(out, colSpan{s + (e-s)*i/n, s + (e-s)*(i+1)/n})
	}
	return out
}
