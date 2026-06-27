package sprite

import "image"

// pingPongStates lists the preset keywords whose loop is generated as an outbound
// HALF cycle and then mirrored into a seamless there-and-back loop (forward, then
// reverse). Only symmetric "held"/oscillating loops belong here — never locomotion
// cycles (walk/run) or circular motions (dizzy), where playing the frames in
// reverse looks wrong. These presets carry a reduced Frames count (the half), and
// the renderer expands them to 2*Frames-2 played frames via PingPongFrames.
var pingPongStates = map[string]bool{
	"idle":        true,
	"idle-combat": true,
	"sleep":       true,
	"meditate":    true,
	"think":       true,
}

// IsPingPong reports whether a state (by its base keyword, direction suffix
// stripped) is generated as a half cycle and mirrored into a ping-pong loop.
func IsPingPong(stateName string) bool {
	return pingPongStates[BaseStateName(stateName)]
}

// PingPongFrames expands an outbound half-cycle into a seamless there-and-back
// loop: the frames, then the interior frames in reverse (the two endpoints are
// not repeated). e.g. [A,B,C] -> [A,B,C,B], which loops cleanly back to A.
// Inputs shorter than 3 frames have no interior to mirror and are returned as-is.
func PingPongFrames(frames []*image.NRGBA) []*image.NRGBA {
	if len(frames) < 3 {
		return frames
	}
	out := make([]*image.NRGBA, 0, 2*len(frames)-2)
	out = append(out, frames...)
	for i := len(frames) - 2; i >= 1; i-- {
		out = append(out, frames[i])
	}
	return out
}
