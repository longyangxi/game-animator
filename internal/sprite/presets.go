package sprite

import "strings"

// PresetInfo defines a single situation keyword (animation preset).
// Hint is used only for server-side prompt generation and is hidden from the frontend via json:"-".
type PresetInfo struct {
	Name     string `json:"name"`     // English state name (for export/file names)
	Label    string `json:"label"`    // display name
	Category string `json:"category"` // category for UI grouping
	Action   string `json:"action"`   // English action description (for the prompt)
	Frames   int    `json:"frames"`   // default frame count
	FPS      int    `json:"fps"`      // default playback speed
	Loop     bool   `json:"loop"`     // default loop flag
	Hint     string `json:"-"`        // motion guide (injected into the prompt, not exposed)
}

// Presets is the catalog of 100 selectable situation keywords.
// This slice is the single source of both the presets (frontend) and the motion hints (backend).
var Presets = []PresetInfo{
	// ── Basics ──
	{"idle", "Idle", "Basics", "subtle breathing idle standing in place", 4, 6, true, "Subtle in-place breathing cycle: gentle chest rise and fall, tiny up-down body shift of a few pixels, occasional blink. Feet stay planted in the same spot in every frame."},
	{"idle-combat", "Combat Idle", "Basics", "ready combat stance, weapon up, weight shifting", 4, 8, true, "Alert combat-ready idle: knees slightly bent, weapon or fists raised, weight shifting subtly side to side, small breathing bob. Feet stay planted; stance never relaxes."},
	{"walk", "Walk", "Basics", "{view} walking cycle facing right", 6, 10, true, "Readable {view} walking cycle: alternating legs with clear contact and passing poses, opposite arm swing, slight body bob. Each frame shows a distinctly different leg position."},
	{"run", "Run", "Basics", "fast {view} running cycle facing right", 6, 12, true, "Fast {view} running cycle: strong forward lean, large leg extension with airborne moments, pumping arms, pronounced body bob. Each frame is a distinct stride phase."},
	{"sprint", "Sprint", "Basics", "all-out sprint, extreme lean and stride", 6, 14, true, "All-out sprint: extreme forward lean, maximal leg extension, both feet airborne at peak, arms pumping hard. Faster, larger strides than a normal run."},
	{"jump", "Jump", "Basics", "crouch, take off, airborne peak, land", 5, 10, false, "Jump sequence: crouching anticipation, take-off with body extended upward, airborne peak with legs tucked, landing recovery crouch. Vary the body's vertical position to show the arc."},
	{"fall", "Fall", "Basics", "falling through the air", 4, 10, true, "Falling cycle: body airborne, arms and legs flailing or bracing, slight rotation or wobble, hair and clothes pushed upward by wind. No ground contact in any frame."},
	{"land", "Land", "Basics", "land from a fall and absorb impact", 4, 12, false, "Landing impact: feet touch down, deep knee bend to absorb shock, body compresses low, then rises back toward standing. Show the compression clearly in the middle frame."},
	{"crouch", "Crouch", "Basics", "lower into a compact crouch and hold", 4, 8, false, "Crouching sequence: from standing, bend knees and lower the body progressively into a compact crouch, head tucked slightly. Final frame is fully crouched."},
	{"crawl", "Crawl", "Basics", "crawl forward on hands and knees", 6, 8, true, "Hands-and-knees crawling cycle: alternating arm-and-opposite-leg reaches, low body close to the ground, head up. Each frame a distinct crawl phase."},
	{"climb", "Climb", "Basics", "climb up a vertical surface", 6, 8, true, "Vertical climbing cycle: alternating hand-over-hand reaches and matching foot pushes, body pressed close to the surface, upward progress implied. Each frame a distinct reach."},
	{"swim", "Swim", "Basics", "swimming stroke cycle", 6, 8, true, "Swimming stroke cycle: arms reaching forward and pulling back in alternation, legs kicking, body horizontal. Each frame a distinct stroke phase."},
	{"dash", "Dash", "Basics", "quick burst dash forward", 4, 14, false, "Quick dash burst: explosive crouch-and-push start, body stretched low and forward at peak speed, then a brief settle. Strong horizontal lean throughout."},
	{"roll", "Roll", "Basics", "evasive forward roll", 5, 14, false, "Evasive forward roll: tuck into a ball, rotate fully over the shoulder, and rise back to a crouch. Show clear rotation phases across frames."},
	{"slide", "Slide", "Basics", "sliding low along the ground", 4, 12, false, "Low slide: drop into a feet-first slide with one leg extended, body leaning back low to the ground, then begin to rise. Body stays low across frames."},
	{"sit", "Sit", "Basics", "sit down to the ground", 4, 8, false, "Sitting down: bend at knees and hips, lower the body, settle onto the ground in a relaxed seated pose. Final frame clearly seated."},
	{"sleep", "Sleep", "Basics", "sleeping lying down, gentle breathing", 4, 4, true, "Sleeping cycle: lying down with eyes closed, slow gentle breathing rise and fall, occasional small shift. Very calm, minimal motion."},
	{"turn", "Turn", "Basics", "turn around to face the other way", 4, 10, false, "Turn-around: rotate the body from facing one way to the opposite, weight pivoting on the feet, head leading the turn. Show clear intermediate angles."},

	// ── Combat ──
	{"attack", "Attack", "Combat", "melee attack with wind-up, strike, recovery", 5, 12, false, "Melee attack: wind-up with body coiled back, powerful strike at full extension, follow-through, recovery to ready stance. The strike frame is the most extreme pose."},
	{"attack-heavy", "Heavy Attack", "Combat", "slow heavy melee attack with big wind-up", 6, 10, false, "Heavy attack: long exaggerated wind-up loading weight back, a slow powerful swing, deep follow-through, slow recovery. Bigger and slower than a normal attack."},
	{"combo", "Combo", "Combat", "multi-hit melee combo", 6, 14, false, "Multi-hit combo: a fast sequence of distinct strikes from different angles (e.g. slash, backslash, thrust), each frame a separate hit, ending in a recovery pose."},
	{"slash", "Slash", "Combat", "horizontal sword slash", 5, 14, false, "Sword slash: coil the blade back, sweep it across in a wide horizontal arc at full extension, follow through to the opposite side, recover. Most extreme pose mid-swing."},
	{"stab", "Stab", "Combat", "forward thrust attack", 4, 14, false, "Thrust attack: draw the weapon back close to the body, explosive straight forward lunge with full arm and weapon extension, then retract. Peak frame fully extended forward."},
	{"punch", "Punch", "Combat", "straight punch", 4, 14, false, "Straight punch: cock the fist back at the hip, drive it forward with shoulder rotation to full extension, retract to guard. Peak frame fully extended."},
	{"kick", "Kick", "Combat", "high kick", 5, 14, false, "High kick: plant and chamber the knee, snap the leg out to full extension, hold the impact pose, retract and settle. Peak frame at maximum leg extension."},
	{"uppercut", "Uppercut", "Combat", "rising uppercut punch", 4, 14, false, "Uppercut: dip the body low loading the legs, drive upward exploding the fist up through the target, finish with body extended tall. Peak frame reaching upward."},
	{"block", "Block", "Combat", "raise guard and hold a defensive block", 3, 10, true, "Defensive block: raise arms or shield to guard, brace with a slight crouch, hold firm with tiny tension shifts. Feet planted, posture steady."},
	{"parry", "Parry", "Combat", "deflect an incoming attack", 4, 16, false, "Parry: a sharp deflecting flick of the weapon or arm to one side that knocks an attack away, then snap back to ready. Quick and crisp."},
	{"dodge", "Dodge", "Combat", "quick sidestep dodge", 4, 16, false, "Dodge: a fast lean-and-step to one side to evade, body weaving out of the way, then recovering balance. Quick lateral motion."},
	{"backstep", "Backstep", "Combat", "quick hop backward", 4, 14, false, "Backstep: a quick defensive hop backward, light push off the front foot, brief airborne drift, land back in guard. Net backward movement."},
	{"shoot", "Shoot", "Combat", "fire a ranged weapon", 4, 14, false, "Ranged shot: steady the weapon, fire with a sharp recoil kick pushing the body back, then settle back on target. Show the recoil clearly. No projectile particles separated from the weapon."},
	{"reload", "Reload", "Combat", "reload a ranged weapon", 5, 10, false, "Reload sequence: lower the weapon, work the mechanism with the off hand (eject, insert, seat), and raise back to ready. Hands do the distinct work across frames."},
	{"aim", "Aim", "Combat", "hold a steady aim down sights", 3, 10, true, "Aiming hold: weapon raised and leveled, body steady and braced, only tiny breathing sway. Posture locked, eyes down the sights."},
	{"throw", "Throw", "Combat", "throw an object overhand", 5, 12, false, "Overhand throw: wind the arm back behind the head, whip it forward releasing at full extension, follow through across the body. Peak frame at release."},
	{"charge-attack", "Charge Attack", "Combat", "charge up then release a powerful attack", 6, 12, false, "Charged attack: a held loading pose gathering power (body coiled, weapon drawn back), then an explosive release strike at full extension, then recovery. Hold the charge for the first frames."},
	{"spin-attack", "Spin Attack", "Combat", "spinning 360 attack", 6, 14, false, "Spin attack: rotate the whole body a full turn while sweeping the weapon around in a wide circle, then settle facing forward. Show distinct rotation angles per frame."},
	{"guard-break", "Guard Break", "Combat", "stagger backward with guard broken", 4, 12, false, "Guard break: the raised guard is smashed open, arms fly apart, body rocks backward off balance, briefly exposed. Recoil reads as defense failing."},
	{"counter", "Counter", "Combat", "absorb a hit then counterattack", 5, 14, false, "Counter: a tight defensive flinch, then an immediate sharp counterattack exploding forward at full extension, then recovery. Two-beat defense-into-offense."},
	{"taunt", "Taunt", "Combat", "taunting gesture toward an enemy", 4, 8, true, "Taunt: a confident provoking gesture — beckoning with a hand, chest puffed, head cocked — looping with attitude. Feet planted, upper body expressive."},
	{"draw-weapon", "Draw Weapon", "Combat", "draw a weapon and enter ready stance", 5, 10, false, "Draw weapon: reach for the weapon, pull it free in a sweeping motion, settle into a ready combat stance. Final frame is the ready pose with weapon up."},

	// ── Magic & Skills ──
	{"cast", "Cast", "Magic & Skills", "generic spell casting", 5, 12, false, "Spell casting: arms gather inward in concentration, then thrust forward in a casting pose, followed by recovery. Pose changes only, no floating magical particles."},
	{"cast-fire", "Fire Cast", "Magic & Skills", "cast a fire spell", 6, 12, false, "Fire spell cast: gather energy at the hands with a coiled stance, then thrust both hands forward releasing the blast. Any flame must be opaque, hard-edged, and touching the hands, not floating particles."},
	{"cast-ice", "Ice Cast", "Magic & Skills", "cast an ice spell", 6, 10, false, "Ice spell cast: a slow controlled gathering pose, hands sweeping inward, then a sharp pointed release forward. Cold, precise, deliberate motion."},
	{"cast-lightning", "Lightning Cast", "Magic & Skills", "cast a lightning spell", 5, 14, false, "Lightning cast: a fast raise of the arm overhead charging, then a sharp downward or forward strike releasing the bolt. Quick and snappy. Effects hard-edged and touching the hand only."},
	{"cast-heal", "Heal Cast", "Magic & Skills", "cast a healing spell on self", 5, 8, false, "Healing cast: bring hands together at the chest in a gentle gathering pose, then open them outward and upward in a soft release, head tilted up. Calm, flowing motion."},
	{"summon", "Summon", "Magic & Skills", "summon by raising arms", 5, 10, false, "Summon: crouch and gather low, then rise sweeping both arms upward and outward in a grand calling gesture, finishing tall with arms raised. Build to the peak."},
	{"channel", "Channel", "Magic & Skills", "channel energy continuously", 4, 8, true, "Channeling loop: a sustained focused pose, hands held out gathering energy, body tense with small pulsing shifts and a slight glow at the hands. Looping concentration."},
	{"buff", "Buff", "Magic & Skills", "self power-up buff gesture", 4, 10, false, "Buff cast: clench fists and pull them inward to the body in a powering-up motion, body tensing and rising slightly, finishing in a strong braced pose."},
	{"shield-up", "Shield Up", "Magic & Skills", "raise a magical shield barrier", 4, 10, false, "Shield up: sweep one arm forward and out to project a barrier, body braced behind it, then hold. Any barrier shape must be opaque and hard-edged."},
	{"teleport", "Teleport", "Magic & Skills", "vanish and reappear", 5, 14, false, "Teleport: body compresses and distorts shrinking away to nothing in the first frames, then reforms and expands back into a solid pose. Use silhouette compression, not particle clouds."},
	{"transform", "Transform", "Magic & Skills", "dramatic transformation", 6, 10, false, "Transformation: a crouched gathering pose, body tensing and shaking, then bursting upward into a new powered-up stance. Build tension then release to a bold final pose."},
	{"power-up", "Power Up", "Magic & Skills", "powering up with surging energy", 5, 10, true, "Power-up loop: braced wide stance, fists clenched, body trembling with effort and energy surging upward, hair and clothes lifting. Looping intensity, feet planted."},
	{"meditate", "Meditate", "Magic & Skills", "sitting meditation, calm breathing", 4, 4, true, "Meditation loop: seated cross-legged, hands resting on knees, eyes closed, very slow calm breathing rise and fall. Minimal serene motion."},
	{"explode", "Explode", "Magic & Skills", "release an explosive burst outward", 5, 16, false, "Explosive release: gather tightly inward, then throw the whole body open releasing a burst outward, then settle. Use the body opening up to imply the blast, effects hard-edged and touching the body."},

	// ── Damage & Status ──
	{"hurt", "Hurt", "Damage & Status", "recoil from being hit", 3, 10, false, "Hit reaction: body recoils backward, head snaps back, brief stagger with arms flailing slightly, then a weakened guard pose. Feet roughly in place."},
	{"hurt-heavy", "Heavy Hurt", "Damage & Status", "stagger hard from a heavy hit", 4, 10, false, "Heavy hit reaction: the whole body is thrown backward and folds from the impact, head whipping back, nearly losing balance, then a struggling recovery. Bigger than a normal hurt."},
	{"knockback", "Knockback", "Damage & Status", "knocked backward through the air", 4, 12, false, "Knockback: launched backward off the feet from a blow, body airborne and tumbling backward, then a hard skidding stop. Clear backward travel through the air."},
	{"knockdown", "Knockdown", "Damage & Status", "knocked down to the ground", 4, 10, false, "Knockdown: struck and losing footing, body rotates and drops, landing flat on the back or side on the ground. Final frame fully down."},
	{"get-up", "Get Up", "Damage & Status", "get up from the ground", 5, 8, false, "Get up: from lying on the ground, push up with the arms, draw the legs under, rise through a crouch back to standing. Clear upward progression."},
	{"stun", "Stun", "Damage & Status", "stunned and wobbling in place", 4, 8, true, "Stunned loop: dazed slumped posture, head lolling, body swaying off balance, knees buckling slightly. Looping wobble, feet barely holding."},
	{"dizzy", "Dizzy", "Damage & Status", "dizzy with head spinning", 4, 8, true, "Dizzy loop: head rolling in circles, body wobbling, arms loose and drifting for balance, unfocused. Looping disorientation. No floating star particles."},
	{"frozen", "Frozen", "Damage & Status", "frozen stiff and trembling", 3, 6, true, "Frozen loop: body locked rigid mid-pose, arms clamped to the sides, only a tiny brittle tremble. Stiff and immobile, shivering slightly."},
	{"burning", "Burning", "Damage & Status", "on fire, flinching from flames", 4, 12, true, "Burning loop: flinching and writhing, patting at the body, hopping in discomfort. Any flame must be opaque, hard-edged, and touching the body, not floating particles."},
	{"poisoned", "Poisoned", "Damage & Status", "sickened by poison, hunched", 4, 6, true, "Poisoned loop: hunched and queasy, clutching the stomach, swaying weakly, head drooping. Looping sickly weakness."},
	{"stagger", "Stagger", "Damage & Status", "stumble and barely keep balance", 4, 10, false, "Stagger: lurch off balance, arms windmilling to recover, feet shuffling to catch the body, then steadying. Reads as almost falling."},
	{"death", "Death", "Damage & Status", "stagger, collapse, lie flat on the ground", 5, 8, false, "Defeat sequence: stagger, collapse to the knees, fall further down, finally lying flat on the ground. Final frame clearly lying down."},
	{"death-fall", "Falling Death", "Damage & Status", "fall backward and collapse", 4, 8, false, "Falling death: thrown backward, arms flung out, body arcing back and dropping, landing flat and motionless. Final frame fully down and still."},
	{"revive", "Revive", "Damage & Status", "rise back to life from the ground", 6, 8, false, "Revive: from lying flat, the body stirs, lifts, and rises through a kneeling pose back to a strong standing stance, head lifting last. Gradual return of strength."},
	{"low-hp", "Low HP", "Damage & Status", "near death, weak and hunched", 4, 6, true, "Low-HP loop: hunched and exhausted, one hand braced on a knee, heavy labored breathing, slight unsteady sway. Barely standing, looping fatigue."},
	{"defeat", "Defeat", "Damage & Status", "drop to knees in defeat", 4, 8, false, "Defeat: shoulders sag, the body sinks down onto the knees, head bowing low in surrender. Ends kneeling and dejected."},

	// ── Emotion ──
	{"wave", "Wave", "Emotion", "friendly hand wave, body still", 4, 8, true, "Friendly greeting: one arm raises and waves side to side across frames while the rest of the body stays still. Hand in clearly different positions each frame. Feet planted."},
	{"cheer", "Cheer", "Emotion", "cheer with arms raised", 4, 10, true, "Cheering loop: throw both arms up overhead repeatedly with a small hop or bounce, head up, joyful. Energetic looping celebration."},
	{"clap", "Clap", "Emotion", "clapping hands", 4, 10, true, "Clapping loop: bring both hands together and apart in front of the chest repeatedly, slight body bounce. Hands clearly open and closed across frames."},
	{"bow", "Bow", "Emotion", "respectful bow", 4, 8, false, "Bow: from standing, bend forward at the waist into a respectful bow, hold briefly, then rise back up. Show the full forward bend."},
	{"nod", "Nod", "Emotion", "nodding the head yes", 3, 8, false, "Nod: tip the head down and back up in agreement, small body settle. Head clearly moves down then up. Body otherwise still."},
	{"shake-head", "Shake Head", "Emotion", "shaking the head no", 4, 8, false, "Head shake: turn the head left and right in refusal, shoulders slightly tense. Head clearly rotates side to side. Body otherwise still."},
	{"laugh", "Laugh", "Emotion", "laughing happily", 4, 8, true, "Laughing loop: head tipped back, shoulders bouncing with laughter, maybe a hand to the belly, big smile. Looping bounce of joy."},
	{"cry", "Cry", "Emotion", "crying sadly", 4, 6, true, "Crying loop: hands toward the face, shoulders shaking with sobs, head bowed, body hunched. Looping sad tremble. Tears optional but must be small and on the face."},
	{"angry", "Angry", "Emotion", "furious, fists clenched", 4, 8, true, "Angry loop: fists clenched, shoulders raised and tense, body trembling with rage, leaning forward, brows down. Looping fury, feet planted."},
	{"surprised", "Surprised", "Emotion", "startled and recoiling", 3, 12, false, "Surprise: a sharp startled jolt — body snaps upright and back, arms fly up, head rears, eyes wide. Quick recoil then a frozen shocked pose."},
	{"think", "Think", "Emotion", "thinking with hand on chin", 4, 6, true, "Thinking loop: one hand to the chin, head tilted, weight shifting slowly side to side, occasional small head tilt. Pondering, looping subtle motion."},
	{"point", "Point", "Emotion", "point forward decisively", 4, 10, false, "Pointing: draw the arm back then thrust it forward extending one finger to point decisively, body leaning into it, then hold. Peak frame fully extended forward."},
	{"salute", "Salute", "Emotion", "military salute", 4, 8, false, "Salute: snap one hand up to the brow in a crisp military salute, body straightening to attention, hold, then lower. Sharp and formal."},
	{"dance", "Dance", "Emotion", "rhythmic dancing", 6, 10, true, "Dancing loop: rhythmic full-body movement — hips and arms swaying, weight shifting foot to foot, head bobbing to a beat. Distinct fun poses per frame, looping smoothly."},
	{"victory", "Victory", "Emotion", "victory pose celebration", 4, 8, true, "Victory loop: a triumphant pose — fist pumped or arms raised, chest out, small confident bounce. Looping celebration, proud and energetic."},
	{"sad", "Sad", "Emotion", "sad and downcast", 4, 4, true, "Sad loop: shoulders slumped, head down, arms hanging limp, a slow heavy sway and sigh. Looping melancholy, minimal motion."},
	{"scared", "Scared", "Emotion", "frightened and cowering", 4, 8, true, "Scared loop: cowering back, arms raised defensively in front of the face, body trembling and shrinking, knees together. Looping fear."},
	{"yawn", "Yawn", "Emotion", "yawning sleepily", 4, 6, false, "Yawn: a big stretch with arms rising and head tilting back, mouth wide in a yawn, then arms lower and shoulders settle. One clear stretch-and-relax."},

	// ── Interaction ──
	{"pick-up", "Pick Up", "Interaction", "bend down and pick up an item", 4, 10, false, "Pick up: bend at the knees and waist down toward the ground, close the hand as if grasping an item, then rise back up holding it. Clear down-then-up motion."},
	{"carry", "Carry", "Interaction", "walk while carrying a load", 6, 8, true, "Carrying walk loop: walking cycle with both arms held forward or up bearing a load, slightly leaned back to balance the weight, shorter steps. Looping burdened walk."},
	{"push", "Push", "Interaction", "push a heavy object forward", 6, 8, true, "Pushing loop: leaning hard forward with both arms extended against an object, legs driving with alternating steps, straining. Looping effortful push."},
	{"pull", "Pull", "Interaction", "pull a heavy object backward", 6, 8, true, "Pulling loop: leaning back with both arms drawn in gripping something, legs stepping backward and digging in, straining. Looping effortful pull."},
	{"open", "Open", "Interaction", "open a door or chest", 4, 10, false, "Open: reach forward toward a handle, grip and pull or push it open with a turning motion, lean in. Clear reach-and-open action with the arm doing the work."},
	{"eat", "Eat", "Interaction", "eating food", 4, 8, false, "Eating: raise a hand to the mouth as if holding food, take a bite with a small head tilt, lower the hand, chew. Clear hand-to-mouth motion."},
	{"drink", "Drink", "Interaction", "drinking from a cup", 4, 8, false, "Drinking: raise a hand to the mouth as if holding a cup, tip the head back to drink, then lower. Clear raise-tip-lower motion."},
	{"read", "Read", "Interaction", "reading a held book", 4, 6, true, "Reading loop: both hands held out front as if holding an open book, head tilted down scanning, occasional small head shift or page turn. Looping calm study."},
	{"dig", "Dig", "Interaction", "digging with a shovel", 6, 8, true, "Digging loop: thrust a shovel down into the ground, scoop, lift and toss the dirt aside, return. Looping dig cycle with clear down-scoop-toss phases."},
	{"mine", "Mine", "Interaction", "swinging a pickaxe to mine", 6, 10, true, "Mining loop: raise a pickaxe overhead, swing it down hard into rock with an impact recoil, lift back up. Looping swing cycle with a clear strike frame."},
	{"chop", "Chop", "Interaction", "chopping with an axe", 6, 10, true, "Chopping loop: raise an axe up and back, swing it down into a target with an impact jolt, recover up. Looping chop cycle with a clear strike frame."},
	{"fish", "Fish", "Interaction", "fishing, cast and wait", 5, 6, true, "Fishing loop: holding a rod out front, a slow gentle bob of the line and small body sway while waiting, occasional tiny tug check. Looping patient wait."},
}

// presetByName is a name → preset index for fast lookup.
var presetByName = func() map[string]PresetInfo {
	m := make(map[string]PresetInfo, len(Presets))
	for _, p := range Presets {
		m[p.Name] = p
	}
	return m
}()

// ListPresets returns the catalog of 100 situation keywords (Hint excluded, json:"-").
func ListPresets() []PresetInfo {
	out := make([]PresetInfo, len(Presets))
	copy(out, Presets)
	return out
}

// PresetByName returns the preset matching the name.
func PresetByName(name string) (PresetInfo, bool) {
	p, ok := presetByName[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// MotionHint returns the motion guide for a state name (catalog-based).
// 8-direction sets carry a direction suffix on the state name, e.g. "attack-south" /
// "attack-north-east", so if there is no exact match it strips the direction suffix
// and looks up the base keyword again.
func MotionHint(stateName string) string {
	key := strings.ToLower(strings.TrimSpace(stateName))
	if p, ok := presetByName[key]; ok {
		return p.Hint
	}
	if base := stripDirectionSuffix(key); base != key {
		if p, ok := presetByName[base]; ok {
			return p.Hint
		}
	}
	return ""
}

// stripDirectionSuffix removes a trailing direction-key suffix ("-south", etc.) from a state name.
// It checks compound direction keys ("-south-east") before single keys ("-east")
// to avoid "attack-south-east" being incorrectly trimmed to "attack-south".
func stripDirectionSuffix(name string) string {
	// Pass 1: compound keys (contain a hyphen, e.g. south-east)
	for _, d := range Directions {
		if strings.Contains(d.Key, "-") {
			if suffix := "-" + d.Key; strings.HasSuffix(name, suffix) {
				return strings.TrimSuffix(name, suffix)
			}
		}
	}
	// Pass 2: single keys (south/north/east/west)
	for _, d := range Directions {
		if !strings.Contains(d.Key, "-") {
			if suffix := "-" + d.Key; strings.HasSuffix(name, suffix) {
				return strings.TrimSuffix(name, suffix)
			}
		}
	}
	return name
}

// BaseStateName lowercases, trims, and strips a trailing direction suffix from a
// state name, yielding the base preset keyword (e.g. "attack-south" -> "attack").
func BaseStateName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	return stripDirectionSuffix(key)
}
