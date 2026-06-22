package sprite

import (
	"image"
	"testing"
)

// 슬라이서는 한 포즈의 검(몸통에 연결된)이 옆 포즈의 컬럼으로 뻗어 컷을 가로질러도,
// 그 검을 옆 프레임으로 흘려보내면 안 된다(A1 bleed). 여기서는 왼쪽 포즈 A만 검을 들고
// 오른쪽 포즈 B는 검이 없으므로, 프레임 B 안의 "검 색" 픽셀은 곧 bleed의 증거다.
//
// 색: 몸통 G=100, 검 G=220. 프레임 B에서 G>=180 인 불투명 픽셀 = 흘러든 검.
func countSwordPixels(img *image.NRGBA) int {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i+3] > alphaThreshold && img.Pix[i+1] >= 180 {
				n++
			}
		}
	}
	return n
}

func TestSwordStaysWithItsPose(t *testing.T) {
	// 실제 strip 토폴로지를 재현: 두 포즈 사이에 빈 컬럼(gutter)이 없어(=검이 컬럼을
	// 채워 projection 골이 사라짐) 강제 컷이 검을 관통하고, 잘려나간 검끝이 옆 프레임에
	// "몸통과 분리된 조각"으로 떨어진다. 단 그 검은 포즈 A에만 연결돼 있다(A1, 수복 가능).
	strip := image.NewNRGBA(image.Rect(0, 0, 380, 100))
	// Pose A: 몸통 + 오른쪽으로 길게 뻗는 검(몸통 A에 연결, 끝 x=300).
	fillBox(strip, 40, 20, 120, 80, 200, 100, 50)  // body A (G=100), rows 20..79
	fillBox(strip, 120, 46, 300, 52, 220, 220, 80) // sword A (G=220), rows 46..51, 몸통과 x=120 연결
	// Pose B: 몸통만(검 없음). 검과 같은 컬럼(250..300)을 공유하되 행이 달라(20..43)
	// 검(46..51)과 닿지 않는다 → 빈 컬럼은 없지만(=골 없음) 검은 B에 연결되지 않는다.
	fillBox(strip, 250, 20, 330, 44, 50, 100, 200) // body B (G=100), rows 20..43

	res := ExtractFrames(strip, 2, 128, 128, 8)
	if len(res.Frames) != 2 {
		t.Fatalf("frames=%d want 2; warnings=%v", len(res.Frames), res.Warnings)
	}

	swordA := countSwordPixels(res.Frames[0])
	swordB := countSwordPixels(res.Frames[1])
	t.Logf("frame A sword pixels=%d (expected >0, A owns the sword)", swordA)
	t.Logf("frame B sword pixels=%d (expected 0, B has no sword)", swordB)

	if swordA == 0 {
		t.Errorf("frame A는 자기 검을 가지고 있어야 한다(테스트 구성 오류 가능)")
	}
	if swordB > 0 {
		t.Errorf("A1 bleed: 검 없는 포즈 B 프레임에 검 픽셀 %d개가 흘러듦", swordB)
	}
}
