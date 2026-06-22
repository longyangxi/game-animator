package sprite

import (
	"bytes"
	"image"

	"github.com/kettek/apng"
)

// EncodeAPNG encodes the frames into an APNG.
// Unlike GIF, it supports 8-bit full alpha so edges do not break up.
func EncodeAPNG(frames []*image.NRGBA, fps int, loop bool) ([]byte, error) {
	if fps <= 0 {
		fps = 8
	}
	a := apng.APNG{}
	if !loop {
		a.LoopCount = 1
	}
	for _, f := range frames {
		a.Frames = append(a.Frames, apng.Frame{
			Image:            f,
			DelayNumerator:   1,
			DelayDenominator: uint16(fps),
		})
	}
	var buf bytes.Buffer
	if err := apng.Encode(&buf, a); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
