package sprite

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

// Create a synthetic strip: magenta background + count solid-color character blobs
func makeSyntheticStrip(w, h, count int, blobColor color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Rect, &image.Uniform{color.NRGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)

	slotW := w / count
	for i := 0; i < count; i++ {
		// rectangular blob (with slightly different heights) at the center of each slot
		bw, bh := slotW/2, h/2+i*4
		x0 := i*slotW + (slotW-bw)/2
		y0 := h - 20 - bh
		draw.Draw(img, image.Rect(x0, y0, x0+bw, y0+bh), &image.Uniform{blobColor}, image.Point{}, draw.Src)
	}
	return img
}

func TestDetectBackground(t *testing.T) {
	strip := makeSyntheticStrip(800, 200, 4, color.NRGBA{40, 80, 200, 255})
	bg := DetectBackground(strip)
	if bg[0] < 200 || bg[1] > 50 || bg[2] < 200 {
		t.Fatalf("magenta background detection failed: %v", bg)
	}
}

func TestRemoveBackgroundAndExtract(t *testing.T) {
	for _, count := range []int{1, 4, 6, 8} {
		strip := makeSyntheticStrip(1200, 260, count, color.NRGBA{40, 80, 200, 255})
		clean := RemoveBackground(strip)

		// the background must be transparent
		if clean.Pix[3] != 0 {
			t.Fatalf("[%d] corner pixel is not transparent", count)
		}

		res := ExtractFrames(clean, count, 256, 256, 24)
		if res.Found != count {
			t.Fatalf("[%d] frame extraction failed: found=%d warnings=%v", count, res.Found, res.Warnings)
		}
		for i, f := range res.Frames {
			if f.Rect.Dx() != 256 || f.Rect.Dy() != 256 {
				t.Fatalf("[%d] frame %d cell size error: %v", count, i, f.Rect)
			}
			// the frame must have actual content
			has := false
			for p := 3; p < len(f.Pix); p += 4 {
				if f.Pix[p] > 128 {
					has = true
					break
				}
			}
			if !has {
				t.Fatalf("[%d] frame %d is empty", count, i)
			}
		}
	}
}

func TestExtractPreservesVerticalOffset(t *testing.T) {
	// jump arc: the second blob floats higher
	w, h := 800, 300
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Rect, &image.Uniform{color.NRGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)
	blue := color.NRGBA{40, 80, 200, 255}
	// blob 1: ground
	draw.Draw(img, image.Rect(100, 200, 180, 280), &image.Uniform{blue}, image.Point{}, draw.Src)
	// blob 2: airborne (100px above the ground)
	draw.Draw(img, image.Rect(500, 100, 580, 180), &image.Uniform{blue}, image.Point{}, draw.Src)

	clean := RemoveBackground(img)
	res := ExtractFrames(clean, 2, 256, 256, 24)
	if res.Found != 2 {
		t.Fatalf("frame extraction failed: %d", res.Found)
	}

	bottomOf := func(f *image.NRGBA) int {
		for y := f.Rect.Dy() - 1; y >= 0; y-- {
			for x := 0; x < f.Rect.Dx(); x++ {
				if f.Pix[f.PixOffset(x, y)+3] > 128 {
					return y
				}
			}
		}
		return -1
	}
	b0, b1 := bottomOf(res.Frames[0]), bottomOf(res.Frames[1])
	if b1 >= b0 {
		t.Fatalf("vertical offset not preserved: ground=%d airborne=%d (airborne should be higher)", b0, b1)
	}
}

func TestDespillRemovesMagentaFringe(t *testing.T) {
	// case where a magenta fringe is mixed into the character's edge
	w, h := 400, 200
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Rect, &image.Uniform{color.NRGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(150, 50, 250, 150), &image.Uniform{color.NRGBA{40, 200, 80, 255}}, image.Point{}, draw.Src)
	// fringe line (green with a magenta tint mixed in)
	draw.Draw(img, image.Rect(148, 50, 150, 150), &image.Uniform{color.NRGBA{180, 120, 190, 255}}, image.Point{}, draw.Src)

	clean := RemoveBackground(img)
	// if the fringe pixel survived, its magenta bias should be reduced
	for y := 50; y < 150; y++ {
		i := clean.PixOffset(148, y)
		if clean.Pix[i+3] == 0 {
			continue // OK if it was removed
		}
		r, g, b := int(clean.Pix[i]), int(clean.Pix[i+1]), int(clean.Pix[i+2])
		if (r+b)/2-g > 80 {
			t.Fatalf("despill not applied: r=%d g=%d b=%d", r, g, b)
		}
	}
}

func TestComposeAtlasAndManifest(t *testing.T) {
	mk := func(n int) []*image.NRGBA {
		var out []*image.NRGBA
		for i := 0; i < n; i++ {
			f := image.NewNRGBA(image.Rect(0, 0, 64, 64))
			draw.Draw(f, image.Rect(10, 10, 50, 50), &image.Uniform{color.NRGBA{200, 100, 50, 255}}, image.Point{}, draw.Src)
			out = append(out, f)
		}
		return out
	}
	states := []StateFrames{
		{Spec: StateSpec{Name: "idle", FPS: 6, Loop: true}, Frames: mk(4)},
		{Spec: StateSpec{Name: "attack", FPS: 12, Loop: false}, Frames: mk(6)},
	}
	sheet, manifest := ComposeAtlas("hero", states, 64, 64)
	if sheet.Rect.Dx() != 6*64 || sheet.Rect.Dy() != 2*64 {
		t.Fatalf("sheet size error: %v", sheet.Rect)
	}
	if manifest.Animations["attack"].Row != 1 || len(manifest.Animations["attack"].Rects) != 6 {
		t.Fatalf("manifest error: %+v", manifest.Animations["attack"])
	}
	if manifest.Animations["idle"].FPS != 6 || !manifest.Animations["idle"].Loop {
		t.Fatalf("idle manifest error")
	}
}

func TestEncodeGIF(t *testing.T) {
	var frames []*image.NRGBA
	for i := 0; i < 4; i++ {
		f := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		draw.Draw(f, image.Rect(i*10, 10, i*10+20, 40), &image.Uniform{color.NRGBA{uint8(50 * i), 120, 220, 255}}, image.Point{}, draw.Src)
		frames = append(frames, f)
	}
	data, err := EncodeGIF(frames, 8, true)
	if err != nil {
		t.Fatalf("GIF encoding failed: %v", err)
	}
	if len(data) < 100 || string(data[:6]) != "GIF89a" {
		t.Fatalf("GIF format error (len=%d)", len(data))
	}
}

func TestBuildStripPrompt(t *testing.T) {
	p := BuildStripPrompt("blue wizard", StylePresets["pixel"], StateSpec{
		Name: "walk", Frames: 6, FPS: 10, Loop: true, Action: "walking",
	}, "make arms bigger")
	for _, want := range []string{"exactly 6", "blue wizard", "magenta", "loops", "make arms bigger", "32-64px game sprite", "limited palette"} {
		if !containsFold(p, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func containsFold(haystack, needle string) bool {
	h, n := []rune(haystack), []rune(needle)
	lower := func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}
outer:
	for i := 0; i+len(n) <= len(h); i++ {
		for j := range n {
			if lower(h[i+j]) != lower(n[j]) {
				continue outer
			}
		}
		return true
	}
	return false
}
