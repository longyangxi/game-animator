package sprite

import (
	"fmt"
	"strings"
)

// isIso reports whether the perspective selects the 2.5D isometric view.
// Empty string and "flat" both mean the default 2D view.
func isIso(perspective string) bool {
	return strings.EqualFold(strings.TrimSpace(perspective), "iso")
}

// perspectiveCharacterBullet returns the framing bullet for the base-character prompt.
// The flat branch returns the exact text the prompt used before this parameter existed.
func perspectiveCharacterBullet(perspective string) string {
	if isIso(perspective) {
		return "- Classic isometric / top-down game view: the camera looks DOWN at the character at about 35 degrees, the way a hero is drawn in an isometric RPG (e.g. Diablo, a tactics RPG). The character faces the viewer HEAD-ON and SYMMETRIC — squared to the camera, NOT turned or rotated to the left or right (this is the canonical front reference). The TOPS of the head, shoulders and feet are clearly visible and the body reads as seen slightly from above (legs and feet gently foreshortened downward); the feet rest on an implied isometric ground plane — do NOT paint any floor, shadow or scenery, the background stays the flat keying color. This is NOT an eye-level front view and NOT turned to a side. Keep it orthographic pixel-isometric: parallel projection at one constant scale, no vanishing-point perspective, no 3D render.\n"
	}
	return "- Almost flat 2D game-sprite view; avoid dramatic perspective, foreshortening, cinematic camera angles, and illustration-style posing.\n"
}

// perspectiveStripClause returns an extra view-lock clause for the per-state strip prompt.
// It is empty for flat so the flat strip prompt is unchanged.
func perspectiveStripClause(perspective string) string {
	if isIso(perspective) {
		return "Camera lock — THIS OVERRIDES THE DEFAULT SPRITE VIEW: draw every pose with the camera looking DOWN at the character at about 35 degrees, exactly like the attached reference and like a hero sprite in an isometric / top-down RPG. The tops of the head, shoulders and feet are visible and the body reads as seen slightly from above, standing on an implied isometric ground plane (never paint a floor, shadow or scenery — the background stays the flat keying color). This is NOT a flat side-on platformer or fighting-game profile, and NOT a flat eye-level front view: do not flatten to eye level for the action — hold the exact same downward overhead tilt in every pose, only the limbs and body move within it. Keep it orthographic pixel-isometric: parallel projection, no vanishing-point perspective, no 3D render.\n\n"
	}
	return ""
}

// ViewToken is the single per-perspective view word substituted into preset
// action and choreography text wherever the "{view}" placeholder appears. It is
// the one place the camera view is named, so presets stay view-agnostic. The
// flat value reproduces the wording presets used before the perspective toggle,
// keeping 2D output byte-identical.
func ViewToken(perspective string) string {
	if isIso(perspective) {
		return "isometric"
	}
	return "side-view"
}

// viewPlaceholder is the token presets use in place of a hardcoded view word.
const viewPlaceholder = "{view}"

// substituteView resolves the {view} placeholder in preset text for the given
// perspective. It always runs; for flat it yields the original wording.
func substituteView(s, perspective string) string {
	return strings.ReplaceAll(s, viewPlaceholder, ViewToken(perspective))
}

// stripBakedFacing removes a hardcoded travel direction from preset action text
// (e.g. walk/run "... facing right"). Callers apply it only when an explicit
// Facing direction is selected, so the Facing direction lock — not a baked
// phrase — owns orientation and never contradicts the chosen direction.
func stripBakedFacing(s string) string {
	for _, p := range []string{" facing right", " facing left"} {
		s = strings.ReplaceAll(s, p, "")
	}
	return s
}

// isoFacingFlatPhrases are flat-camera wordings baked into the directional
// facing descriptions ("at eye level", a strict 2D profile) that contradict the
// isometric downward tilt. They are stripped — but only in iso mode.
var isoFacingFlatPhrases = []string{
	", at eye level",
	"; strictly 2D profile, no perspective rotation",
}

// isoNeutralize removes flat-camera phrasing from a directional facing section.
// Callers invoke it ONLY when isIso is true; the flat path never touches the
// text, so flat output stays byte-identical (locked by the golden tests).
func isoNeutralize(s string) string {
	for _, p := range isoFacingFlatPhrases {
		s = strings.ReplaceAll(s, p, "")
	}
	return s
}

// isoFacingTilt is the per-direction isometric tilt line appended to a facing
// section in iso mode. Centralized here so all perspective wording lives in one
// place; direction.go calls it.
func isoFacingTilt() string {
	return "- Isometric tilt: on top of the facing above, the camera looks DOWN at the character at about 35 degrees (not eye level) — tops of head, shoulders and feet visible, the body slightly foreshortened from above on an implied isometric ground plane, the same downward tilt in every frame.\n"
}

// StylePresets is the set of selectable style contracts.
var StylePresets = map[string]string{
	"pixel": "true low-resolution pixel-art game sprite, like a 32-64px sprite enlarged on the canvas, " +
		"chunky readable silhouette, clean dark 1px outline, visible square pixel blocks, " +
		"grid-aligned hard pixel edges, limited shared palette, solid tone clusters, " +
		"flat color shading with at most one highlight step and one shadow step, " +
		"simple readable face and clearly separated limbs. " +
		"Never use painterly rendering, smooth gradients, airbrush shading, glossy lighting, " +
		"anti-aliased fine detail, high-definition pixel art, fine-grained pixel art, anime illustration, concept art, or 3D rendering.",
	"chibi": "cute chibi game sprite with oversized head and small body, " +
		"bold dark outline, flat bright colors, minimal shading, large expressive eyes, " +
		"clean cartoon shapes readable at small size. " +
		"Never use realistic proportions, gradients, or painterly detail.",
	"cartoon": "clean 2D cartoon game sprite, bold uniform outline, flat vivid colors, " +
		"simple two-tone cel shading, smooth rounded shapes, expressive but simple face. " +
		"Never use pixelation, gradients, photo textures, or 3D rendering.",
	"retro16": "16-bit retro console era game sprite, restrained palette of 16-24 colors, " +
		"dark outline, dithering only where needed, compact proportions, " +
		"crisp hard pixel edges like a classic arcade fighter sprite. " +
		"Never use modern smooth shading or high-resolution detail.",
}

// keyColorPhrase is the keying-background description phrase (the color matting separates).
const keyColorPhrase = "pure keying magenta (#FF00FF), perfectly uniform edge to edge"

// ResolveStyle converts a preset key or custom style text into a style contract.
func ResolveStyle(presetKey, custom string) string {
	if strings.TrimSpace(custom) != "" {
		return strings.TrimSpace(custom)
	}
	if s, ok := StylePresets[presetKey]; ok {
		return s
	}
	return StylePresets["pixel"]
}

// canvasContract returns the keying-canvas rules (the core contract the matting stage relies on).
func canvasContract() string {
	var b strings.Builder
	b.WriteString("Keying canvas (the renderer mattes this away — obey exactly):\n")
	b.WriteString("- Fill the ENTIRE background, edge to edge, with " + keyColorPhrase + " — a single flat color touching all four image borders. No gradient, texture, scenery, floor, panel, frame, or border of any kind.\n")
	b.WriteString("- The subject must avoid magenta, pink and purple entirely — clothing, props, highlights and effects included — so the keyer never eats part of the character.\n")
	b.WriteString("- Drop every shadow and contact patch; the ground is implied, never painted.\n")
	return b.String()
}

// spriteDesignContract locks in the game-sprite structure required by the default pixel style.
func spriteDesignContract() string {
	var b strings.Builder
	b.WriteString("Game-sprite design contract:\n")
	b.WriteString("- Interpret the subject as a game-ready character sprite, not an illustration, poster, sticker, mascot logo, or concept-art render.\n")
	b.WriteString("- Preserve the subject's identity through a strong silhouette, hairstyle, outfit shapes, accessories, weapon or signature prop, and dominant color blocks.\n")
	b.WriteString("- Simplify anatomy into readable sprite shapes: compact torso, clear head shape, simple arms and legs, minimal joint detail, no tiny anatomy rendering.\n")
	b.WriteString("- Hair, clothing layers, capes, hats, weapons and accessories should read as distinct hard-edged pixel shapes, not detailed painted textures.\n")
	b.WriteString("- Keep the face simple at sprite scale: readable eyes and mouth, minimal facial detail, no realistic nose or painted skin texture.\n")
	return b.String()
}

// lowResPixelContract locks in the rendering-resolution feel so the model does not drift into an HD illustration.
func lowResPixelContract() string {
	var b strings.Builder
	b.WriteString("Pixel rendering contract:\n")
	b.WriteString("- The image must look like a 32-64px game sprite enlarged to the canvas, not newly painted at high resolution.\n")
	b.WriteString("- Use chunky square pixel blocks, clean 1px outline, solid tone clusters, limited palette, minimal two-step flat shading.\n")
	b.WriteString("- No dithering, no smooth gradients, no soft shadow, no blur, no airbrush, no texture, no fine hair strands, no tiny jewelry detail that would vanish at 64px.\n")
	b.WriteString("- Every important shape must remain readable when shrunk to a thumbnail: silhouette first, details second.\n")
	return b.String()
}

func pixelStyleContracts(style string) string {
	s := strings.ToLower(style)
	if !strings.Contains(s, "pixel") && !strings.Contains(s, "sprite") && !strings.Contains(s, "mmorpg") {
		return ""
	}
	return spriteDesignContract() + "\n" + lowResPixelContract()
}

// rejectClause is a concise contract rejecting elements that hinder extraction.
func rejectClause() string {
	var b strings.Builder
	b.WriteString("Reject (these break automatic extraction):\n")
	b.WriteString("- ANY frame, border, or decoration around the image or around a pose: no film strip, no sprocket holes or perforations, no photo/polaroid frame, no panel dividers, no outline box, no vignette. The background reaches every edge unbroken.\n")
	b.WriteString("- Motion garnish — streaks, speed lines, blur, after-images, arcs, swooshes, trails.\n")
	b.WriteString("- Free-floating bits — sparkles, stars, dust, smoke puffs, icons, symbols, or any mark not fused to the body.\n")
	b.WriteString("- Text, numbers, captions, grids, rulers, speech or thought bubbles, UI, watermarks.\n")
	b.WriteString("- Any pose that is clipped by the edge, or whose pixels bridge into the neighbouring pose.\n")
	return b.String()
}

// BuildCharacterPrompt builds the text-description → base-character image generation prompt.
func BuildCharacterPrompt(description, style, perspective string) string {
	var b strings.Builder
	b.WriteString("Produce one complete game-character reference sprite in a relaxed player-avatar standing pose.\n\n")
	fmt.Fprintf(&b, "Subject: %s.\n\n", strings.TrimSpace(description))
	b.WriteString("Feature audit before drawing (do this internally, then render): identify and preserve the subject's hairstyle, hair color, eye color, outfit layers, accessories, weapon or signature prop, symbolic motifs, and dominant colors.\n\n")
	fmt.Fprintf(&b, "Render contract (obey strictly): %s\n\n", style)
	if extra := pixelStyleContracts(style); extra != "" {
		b.WriteString(extra)
		b.WriteString("\n")
	}
	b.WriteString("Framing:\n")
	b.WriteString("- A single figure, head to feet, vertically centered, occupying about three quarters of the canvas height with generous breathing room on every side.\n")
	b.WriteString("- Idle standing sprite pose: feet level, weight balanced, arms relaxed but readable.\n")
	b.WriteString(perspectiveCharacterBullet(perspective))
	b.WriteString("- One continuous silhouette — nothing detached, no trailing accessories or particles.\n\n")
	b.WriteString(canvasContract())
	b.WriteString("\n")
	b.WriteString(rejectClause())
	return b.String()
}

// BuildStripPrompt builds the per-state horizontal strip generation prompt.
func BuildStripPrompt(description, style string, spec StateSpec, feedback, perspective string) string {
	var b strings.Builder
	n := spec.Frames
	rows, cols := GridForFrames(n)

	if rows > 1 {
		fmt.Fprintf(&b, "Draw exactly %d game-sprite poses of one character for the \"%s\" animation, laid out in a grid of %d columns by %d rows, read left to right then top to bottom (pose 1 top-left, the next finishing the top row, then continuing on the second row). This is raw sprite art, not a photo or a film — draw only the character poses on a flat background.\n\n", n, spec.Name, cols, rows)
	} else {
		fmt.Fprintf(&b, "Draw a single horizontal row of exactly %d game-sprite poses of one character for the \"%s\" animation, ordered left to right. This is raw sprite art, not a photo or a film — draw only the character poses on a flat background.\n\n", n, spec.Name)
	}

	b.WriteString("Subject lock (top priority):\n")
	b.WriteString("- The attached image is the canonical character. Match it exactly across every pose: face, hairstyle, build, outfit, accessories.\n")
	b.WriteString("- Palette is binding. Re-sample each region's hue, saturation and value from the reference — skin, hair, every garment, every piece of gear. Do not re-tint, re-light, brighten, darken, or substitute a similar shade.\n")
	b.WriteString("- Hold one fixed camera and facing. The figure never rotates, mirrors, ages, or restyles between poses — only the body moves.\n")
	if isIso(perspective) {
		b.WriteString("- That fixed camera is the reference's high three-quarter overhead view (tilted down about 35 degrees); never flatten to a side-on view for the action.\n")
	}
	b.WriteString("\n")

	if d := strings.TrimSpace(description); d != "" {
		fmt.Fprintf(&b, "Subject notes: %s.\n\n", d)
	}
	fmt.Fprintf(&b, "Render contract (obey strictly): %s\n\n", style)
	if extra := pixelStyleContracts(style); extra != "" {
		b.WriteString(extra)
		b.WriteString("\n")
	}

	b.WriteString(perspectiveStripClause(perspective))

	if sec := FacingPromptSection(spec.Facing, perspective); sec != "" {
		b.WriteString(sec)
		b.WriteString("\n")
	}

	action := strings.TrimSpace(spec.Action)
	if action == "" {
		action = spec.Name
	}
	hint := MotionHint(spec.Name)
	// Resolve the {view} placeholder (e.g. walk/run framing) to the perspective's
	// view word. Always runs; for flat it reproduces the original wording.
	action = substituteView(action, perspective)
	hint = substituteView(hint, perspective)
	// When an explicit facing is chosen, the Facing direction lock owns
	// orientation — drop any baked "facing right/left" so it can't contradict.
	if spec.Facing != "" {
		action = stripBakedFacing(action)
	}
	fmt.Fprintf(&b, "Movement: %s.\n", action)
	if hint != "" {
		fmt.Fprintf(&b, "Choreography: %s\n", hint)
	}
	fmt.Fprintf(&b, "Treat the %d poses as evenly timed beats of one continuous motion — pose k is phase k of %d, and neighbours read as smooth in-betweens, never unrelated stances.\n", n, n)
	if spec.Loop {
		b.WriteString("It loops: the final pose must hand off cleanly into the first.\n\n")
	} else {
		b.WriteString("It plays once: give it a clear start, peak, and settle.\n\n")
	}

	if rows > 1 {
		b.WriteString("Grid layout:\n")
		fmt.Fprintf(&b, "- Arrange exactly %d poses in a clean %d-column by %d-row grid, read left to right then top to bottom — %d poses, no more and no fewer. Count them before finishing.%s\n", n, cols, rows, n, lastCellNote(n, rows*cols))
		b.WriteString("- Treat each grid slot as its own cell with the pose centered inside it. Every pose is the SAME size at one shared scale, each filling about 70-85% of its cell. No pose may be noticeably smaller, larger, or set further back than the others.\n")
		b.WriteString("- Leave a generous band of the flat keying background BETWEEN ROWS and BETWEEN COLUMNS, wide enough that each cell is clearly separate. Nothing — not a blade, shield, limb, or trailing effect — may cross into a neighbouring cell or the gap.\n")
		b.WriteString("- Each pose is ONE whole, connected body. Never split a body into separate pieces, and never let two poses touch, overlap, or merge.\n")
		b.WriteString("- Keep each pose's whole reach — weapon swing, extended limbs, shield — INSIDE its own cell. If a swing would leave the cell, angle or foreshorten it so its tip stays within the cell.\n")
		b.WriteString("- Within each row, keep all poses standing on one common ground line, unless the action leaves the ground (a jump).\n\n")
	} else {
		b.WriteString("Row layout:\n")
		fmt.Fprintf(&b, "- Place exactly %d poses in one horizontal row, evenly spaced left to right — %d poses, no more and no fewer. Count them before finishing.\n", n, n)
		b.WriteString("- Every pose is the SAME size at one shared scale, each filling about 70-85% of the canvas height. No pose may be noticeably smaller, larger, or set further back than the others.\n")
		b.WriteString("- Leave a generous band of the flat keying background between every pair of poses. The gap must be wide enough that a human can easily see each pose is separate — never touching, overlapping, or bridging.\n")
		b.WriteString("- Each pose is ONE whole, connected body. Never split a body into separate pieces, and never let two poses touch, overlap, or merge.\n")
		b.WriteString("- Center each pose's torso horizontally in its share of the row; arms, legs and head move, but the torso stays put and no body part is cut off by the canvas edge.\n")
		b.WriteString("- Keep all poses standing on one common ground line, unless the action leaves the ground (a jump).\n")
		b.WriteString("- When the body leans or reaches far to one side, keep the torso/hips within the pose's column so that poses do not bridge into the next gap.\n\n")
	}

	b.WriteString(canvasContract())
	b.WriteString("\n")
	b.WriteString(rejectClause())
	b.WriteString("- Favor changes of pose, weight and expression over decoration; any effect must be opaque, hard-edged, and fused to the body.\n")
	b.WriteString("- Keep every pose legible at thumbnail size: bold silhouette, clear limbs, no detail that vanishes when shrunk.\n")

	if f := strings.TrimSpace(feedback); f != "" {
		fmt.Fprintf(&b, "\nArtist revision (apply over everything above): %s\n", f)
	}
	return b.String()
}

// GridForFrames returns the (rows, cols) generation/slicing layout for a pose count. Single row for
// ≤3 frames; a 2-row grid for 4+ so each pose gets a roomy near-square cell (big weapon swings cross
// less) while staying one image (consistent identity, scale, baseline). cols = ceil(n/rows); the last
// row holds the remainder (5→2×3 with one empty cell, etc.).
func GridForFrames(n int) (rows, cols int) {
	if n <= 3 {
		if n < 1 {
			n = 1
		}
		return 1, n
	}
	return 2, (n + 1) / 2
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// lastCellNote tells the model to leave a trailing empty cell blank when the pose count doesn't
// fill the grid (e.g. 5 poses in a 2×3 grid, 7 in a 2×4).
func lastCellNote(n, total int) string {
	if n < total {
		return " The bottom-right cell has no pose — leave it as empty flat background."
	}
	return ""
}

// AspectForFrames picks the generation aspect ratio for the frame count. For a 2-row grid it returns
// cols:rows so each cell is near-square. For a single row (≤3 frames) the width scales with the pose
// count so each pose keeps a roughly constant horizontal share, clamped to [16:9, 36:9]. (Providers
// that only accept a fixed aspect set, e.g. Gemini, snap this down to their nearest supported ratio.)
func AspectForFrames(frames int) string {
	if frames <= 1 {
		return "1:1"
	}
	if rows, cols := GridForFrames(frames); rows > 1 {
		g := gcd(cols, rows)
		return fmt.Sprintf("%d:%d", cols/g, rows/g) // grid → near-square cells (reduced ratio)
	}
	const perPose = 0.6                // each pose's horizontal share, relative to height 1.0
	const minR, maxR = 16.0 / 9.0, 4.0 // clamp width:height to [16:9, 36:9]
	ratio := float64(frames) * perPose
	if ratio < minR {
		ratio = minR
	}
	if ratio > maxR {
		ratio = maxR
	}
	return fmt.Sprintf("%d:9", int(ratio*9+0.5))
}
