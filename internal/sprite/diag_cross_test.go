package sprite

import (
	"image"
	"image/png"
	"os"
	"testing"
)

func componentSizes(img *image.NRGBA) []int {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	seen := make([]bool, w*h)
	op := func(x, y int) bool {
		return x >= 0 && x < w && y >= 0 && y < h && img.Pix[img.PixOffset(x, y)+3] > alphaThreshold
	}
	var sizes []int
	stack := make([][2]int, 0, 256)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if seen[y*w+x] || !op(x, y) {
				continue
			}
			n := 0
			stack = stack[:0]
			stack = append(stack, [2]int{x, y})
			seen[y*w+x] = true
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				n++
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if op(nx, ny) && !seen[ny*w+nx] {
						seen[ny*w+nx] = true
						stack = append(stack, [2]int{nx, ny})
					}
				}
			}
			sizes = append(sizes, n)
		}
	}
	return sizes
}

func largestAndRest(sizes []int) (largest, rest int) {
	for _, s := range sizes {
		if s > largest {
			rest += largest
			largest = s
		} else {
			rest += s
		}
	}
	return
}

// 진단용(임시, CI 무관): 실제로 교차한 strip을 읽어 분할 경계 + 프레임별 연결요소를 본다.
func TestDiagCrossStrip(t *testing.T) {
	const path = "/Users/marklong/.open-office/projects/.images/attack-heavy-raw-CIJEJs.png"
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("strip not present: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	strip := ToNRGBA(img)
	segs, natural := segmentStrip(strip, 6)
	t.Logf("strip %dx%d  natural=%d (want 6)", strip.Rect.Dx(), strip.Rect.Dy(), natural)
	for i, s := range segs {
		t.Logf("  seg[%d] cols [%d,%d) width=%d", i, s.start, s.end, s.end-s.start)
	}
	res := ExtractFrames(strip, 6, 256, 256, 24)
	for i, fr := range res.Frames {
		comps := componentSizes(fr)
		main, rest := largestAndRest(comps)
		t.Logf("  frame %d: %d components, largest=%d, stray=%d", i, len(comps), main, rest)
	}
	for _, w := range res.Warnings {
		t.Logf("  warning: %s", w)
	}
}
