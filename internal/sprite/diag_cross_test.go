package sprite

import (
	"image/png"
	"os"
	"testing"
)

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
