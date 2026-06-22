package sprite

import (
	"image"
	"testing"
)

// 검증용(임시): 몸통은 4프레임 모두 동일(같은 크기·같은 수직 위치·발 고정),
// 오직 검 길이만 프레임마다 다르게 한다. 파이프라인이 안정적이라면 출력 4프레임에서
// "몸통"의 위치/크기가 동일해야 한다. 흔들리면 B1(스케일 맥동)/B2(수평 흔들림)/
// B4(수직 부침)가 실재한다는 증거.
//
// 색으로 몸통/검을 구분: 몸통 G=100, 검 G=220 → G<160 이면 몸통 픽셀.

func bodyBBox(img *image.NRGBA) (minX, minY, maxX, maxY, count int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	minX, minY, maxX, maxY = w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i+3] <= alphaThreshold {
				continue
			}
			if img.Pix[i+1] >= 160 { // G>=160 → 검(노랑) 픽셀, 몸통 아님
				continue
			}
			count++
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
	return
}

func TestVerifyBodyStableUnderSwordSwing(t *testing.T) {
	const cell = 200
	strip := image.NewNRGBA(image.Rect(0, 0, 4*cell, 120))
	swordLen := []int{10, 35, 70, 100} // 프레임마다 검 길이만 변함

	for i := 0; i < 4; i++ {
		base := i * cell
		// 몸통: 모든 프레임 동일한 기하 (50w x 70h, 발 y=95 고정)
		fillBox(strip, base+40, 25, base+90, 95, 200, 100, 50)
		// 검: 같은 손 위치(x=90, y 55..61)에서 오른쪽으로 길이만 다르게
		fillBox(strip, base+90, 55, base+90+swordLen[i], 61, 220, 220, 80)
	}

	segs, natural := segmentStrip(strip, 4)
	t.Logf("natural=%d (want 4), segs=%v", natural, segs)

	res := ExtractFrames(strip, 4, 128, 128, 8)
	if len(res.Frames) != 4 {
		t.Fatalf("frames=%d want 4; warnings=%v", len(res.Frames), res.Warnings)
	}

	type m struct{ left, top, w, h int }
	got := make([]m, 4)
	for i, f := range res.Frames {
		x0, y0, x1, y1, _ := bodyBBox(f)
		got[i] = m{left: x0, top: y0, w: x1 - x0 + 1, h: y1 - y0 + 1}
		t.Logf("frame %d body: left=%d top=%d w=%d h=%d", i, got[i].left, got[i].top, got[i].w, got[i].h)
	}

	// 몸통은 동일해야 한다. 분산을 측정한다.
	rng := func(sel func(m) int) (int, int) {
		lo, hi := got[0].left, got[0].left
		_ = lo
		lo, hi = sel(got[0]), sel(got[0])
		for _, g := range got {
			v := sel(g)
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		return lo, hi
	}
	lw, hw := rng(func(g m) int { return g.w })
	lh, hh := rng(func(g m) int { return g.h })
	ll, hl := rng(func(g m) int { return g.left })
	lt, ht := rng(func(g m) int { return g.top })

	t.Logf("body width  spread: %d..%d (Δ=%d)  ← B1 스케일 맥동", lw, hw, hw-lw)
	t.Logf("body height spread: %d..%d (Δ=%d)  ← B1 스케일 맥동", lh, hh, hh-lh)
	t.Logf("body left   spread: %d..%d (Δ=%d)  ← B2 수평 흔들림", ll, hl, hl-ll)
	t.Logf("body top    spread: %d..%d (Δ=%d)  ← B4 수직 부침", lt, ht, ht-lt)

	if hw-lw > 1 || hh-lh > 1 {
		t.Errorf("B1 확인: 몸통 크기가 프레임마다 다름 (검 길이만 바뀌었는데) wΔ=%d hΔ=%d", hw-lw, hh-lh)
	}
	if hl-ll > 1 {
		t.Errorf("B2 확인: 몸통 수평 위치가 검에 끌려 흔들림 leftΔ=%d", hl-ll)
	}
	if ht-lt > 1 {
		t.Errorf("B4 확인: 몸통 수직 위치가 흔들림 topΔ=%d", ht-lt)
	}
}
