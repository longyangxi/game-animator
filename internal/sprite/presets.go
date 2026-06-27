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
	{"idle", "Idle", "Basics", "a lively idle: breathing, subtle weight shift, small alive motion in place", 3, 6, true, "Characterful idle that reads as alive: clear breathing with the chest and shoulders rising and falling, a subtle weight shift and gentle body sway, the head and hands drifting slightly, an occasional blink. Keep it lively and readable, but the feet stay firmly planted in the exact same spot in every frame."},
	{"idle-combat", "Combat Idle", "Basics", "a tense combat-ready stance, weapon up, weight shifting", 3, 8, true, "Alert combat-ready idle with coiled tension: knees bent in an athletic stance, weapon or fists raised and ready, weight shifting actively side to side, a small breathing bob. Stay charged and dangerous; feet stay planted and the stance never relaxes; loops cleanly."},
	{"walk", "Walk", "Basics", "a confident {view} walking cycle facing right", 6, 10, true, "Readable {view} walking cycle with clear, lively poses: alternating legs hitting distinct contact and passing positions, full opposite-arm swing, a purposeful slight body bob. Each frame a distinctly different, well-defined leg position; the final pose loops cleanly into the first."},
	{"run", "Run", "Basics", "explosive full-speed {view} running cycle facing right", 6, 12, true, "High-energy {view} run cycle with exaggerated, dynamic strides: a strong aggressive forward lean, legs thrown to FULL extension with clear airborne moments mid-stride, arms pumping hard, pronounced body bob. Push each pose to a dramatic extreme — big stride split, deep reach. Every frame is a distinct stride phase and the final pose hands off cleanly into the first."},
	{"sprint", "Sprint", "Basics", "an all-out sprint, extreme lean and stride", 6, 14, true, "All-out sprint pushed to the extreme: a dramatic forward lean almost falling, legs at MAXIMUM extension with both feet airborne at the peak, arms pumping hard. Bigger, faster, more exaggerated strides than a normal run; each frame a distinct explosive stride phase that loops cleanly."},
	{"jump", "Jump", "Basics", "crouch, explosive take-off, airborne peak, land", 5, 10, false, "Dynamic jump arc: a deep crouching anticipation loading the legs, an explosive take-off with the body stretched fully upward, an airborne peak with the legs tucked high, then a landing recovery crouch. Exaggerate the vertical travel so the arc reads clearly across frames."},
	{"fall", "Fall", "Basics", "plummeting through the air", 4, 10, true, "Falling cycle with dynamic motion: body airborne, arms and legs flailing or bracing dramatically, a slight rotation or wobble, hair and clothes whipped upward by the rushing wind. No ground contact in any frame; loops."},
	{"land", "Land", "Basics", "land hard and absorb the impact", 4, 12, false, "Heavy landing impact: feet slam down, a deep knee bend absorbing the shock with the body compressing low, then rising back toward standing. Push the mid-frame compression to an extreme so the impact reads."},
	{"crouch", "Crouch", "Basics", "lower into a compact crouch and hold", 4, 8, false, "Crouching sequence: from standing, bend the knees and sink the body progressively into a low, compact crouch, head tucked and weight gathered. Final frame is fully, deliberately crouched."},
	{"crawl", "Crawl", "Basics", "crawl low on hands and knees", 6, 8, true, "Hands-and-knees crawling cycle: alternating arm-and-opposite-leg reaches with clear extension, body kept low to the ground, head up and watchful. Each frame a distinct, readable crawl phase; loops cleanly."},
	{"climb", "Climb", "Basics", "climb hand-over-hand up a vertical surface", 6, 8, true, "Vertical climbing cycle: strong alternating hand-over-hand reaches with matching foot pushes, body pressed close to the surface, clear upward effort. Each frame a distinct reach; loops cleanly."},
	{"swim", "Swim", "Basics", "powerful swimming stroke cycle", 6, 8, true, "Swimming stroke cycle with full reach: arms reaching far forward and pulling all the way back in alternation, legs kicking, body held horizontal. Each frame a distinct, committed stroke phase; loops cleanly."},
	{"dash", "Dash", "Basics", "an explosive burst dash forward", 4, 14, false, "Explosive dash burst: a coiled crouch-and-push start, the body stretched low and forward at peak speed with a hard horizontal lean, then a brief settle. Exaggerate the forward stretch at the peak."},
	{"roll", "Roll", "Basics", "an evasive forward roll", 5, 14, false, "Evasive forward roll: tuck tightly into a ball, rotate fully over the shoulder with clear momentum, and rise back to a crouch. Show distinct, dynamic rotation phases across the frames."},
	{"slide", "Slide", "Basics", "a low sliding dash along the ground", 4, 12, false, "Low slide: drop into a committed feet-first slide with one leg extended and the body leaning back low to the ground, then begin to rise. Keep the body dramatically low across the slide frames."},
	{"sit", "Sit", "Basics", "sit down to the ground", 4, 8, false, "Sitting down: bend at the knees and hips, lower the body under control, and settle into a relaxed seated pose. Final frame clearly, comfortably seated."},
	{"sleep", "Sleep", "Basics", "sleeping, slow gentle breathing", 3, 4, true, "Calm sleeping cycle: lying down with eyes closed, slow gentle breathing rising and falling, an occasional small shift. Very serene, minimal motion; loops smoothly."},
	{"turn", "Turn", "Basics", "turn around to face the other way", 4, 10, false, "Turn-around: pivot the body from facing one way to the opposite, weight rolling across the feet, the head leading the turn. Show clear intermediate angles so the rotation reads."},

	// ── Combat ──
	{"attack", "Attack", "Combat", "a powerful overhead melee strike: wind up with arms raised high, then a dynamic lunging strike at full extension", 5, 12, false, "Explosive melee attack with exaggerated, dynamic poses: a deep wind-up coiling the whole body back with the weapon or fists raised high, then a powerful lunging strike thrown to FULL extension — deep forward stance, strong forward lean, both arms driving forward — then follow-through and recovery to a ready stance. Make the strike frame the single most extreme, exaggerated pose: maximum reach and a deep lunge."},
	{"attack-heavy", "Heavy Attack", "Combat", "a slow, heavy melee attack with a huge wind-up", 6, 10, false, "Heavy attack with massive, exaggerated poses: a long deep wind-up loading all the weight back with the weapon drawn far behind, a slow powerful swing committing the whole body, a deep follow-through, then a slow recovery. Bigger and slower than a normal attack — make the swing's peak the most extreme reach."},
	{"combo", "Combo", "Combat", "a fast multi-hit melee combo", 6, 14, false, "Multi-hit combo with dynamic, distinct poses: a fast sequence of separate strikes from different angles (e.g. slash, backslash, thrust), each frame a clearly different committed hit at full extension, ending in a recovery pose. Each hit reads as its own extreme pose."},
	{"slash", "Slash", "Combat", "a wide horizontal sword slash", 5, 14, false, "Sword slash with a big dynamic arc: coil the blade far back, sweep it across in a wide horizontal arc at FULL extension, follow through to the opposite side, then recover. The mid-swing frame is the most extreme, fully-extended pose."},
	{"stab", "Stab", "Combat", "a forward thrust at full extension", 4, 14, false, "Thrust attack: draw the weapon back close to the body coiling for power, then an explosive straight forward lunge with FULL arm and weapon extension and a deep front stance, then retract. Peak frame fully extended forward at maximum reach."},
	{"punch", "Punch", "Combat", "a straight punch at full extension", 4, 14, false, "Straight punch: cock the fist back at the hip with the shoulder loaded, drive it forward with full shoulder rotation to FULL extension and a committed stance, then retract to guard. Peak frame fully extended at maximum reach."},
	{"kick", "Kick", "Combat", "a high kick at full extension", 5, 14, false, "High kick: plant and chamber the knee, then snap the leg out to FULL extension in a dynamic high kick, hold the impact pose, retract and settle. Peak frame at maximum leg extension and height."},
	{"uppercut", "Uppercut", "Combat", "a rising uppercut", 4, 14, false, "Uppercut: dip the body low loading the legs with coiled tension, then explode upward driving the fist up through the target, finishing with the body fully extended tall. Peak frame reaching dramatically upward."},
	{"block", "Block", "Combat", "raise a firm defensive guard and hold", 3, 10, true, "Defensive block held firm: raise the arms or shield into a solid guard, brace with a low grounded crouch, hold steady with tiny tension shifts. Strong and immovable; feet planted, posture locked; loops."},
	{"parry", "Parry", "Combat", "a sharp deflecting parry", 4, 16, false, "Parry: a sharp, crisp deflecting flick of the weapon or arm to one side that knocks an attack away, then a fast snap back to ready. Quick and decisive — the deflect frame is sharp and distinct."},
	{"dodge", "Dodge", "Combat", "a quick weaving dodge to the side", 4, 16, false, "Dodge: a fast lean-and-step to one side to evade, the body weaving dramatically out of the way at the peak, then recovering balance. Quick, dynamic lateral motion."},
	{"backstep", "Backstep", "Combat", "a quick hop backward", 4, 14, false, "Backstep: a quick defensive hop backward — a light push off the front foot, a brief airborne drift, then landing back in guard. Net backward movement, light and snappy."},
	{"shoot", "Shoot", "Combat", "fire a ranged weapon with sharp recoil", 4, 14, false, "Ranged shot: steady the weapon, fire with a sharp recoil kick snapping the body back, then settle back on target. Make the recoil read clearly. No projectile particles separated from the weapon — keep any muzzle effect fused to the weapon."},
	{"reload", "Reload", "Combat", "reload a ranged weapon", 5, 10, false, "Reload sequence: lower the weapon, work the mechanism with the off hand (eject, insert, seat) with clear distinct hand positions, then snap back up to ready. The hands do obvious, readable work across the frames."},
	{"aim", "Aim", "Combat", "hold a steady aim down the sights", 3, 10, true, "Aiming hold: weapon raised and leveled, body steady and braced, only a tiny breathing sway. Posture locked and focused, eyes down the sights; loops."},
	{"throw", "Throw", "Combat", "an overhand throw at full extension", 5, 12, false, "Overhand throw: wind the arm far back behind the head coiling the body, then whip it forward releasing at FULL extension with a committed step, follow through across the body. Peak frame at the moment of release, maximum reach."},
	{"charge-attack", "Charge Attack", "Combat", "charge up, then release a powerful attack", 6, 12, false, "Charged attack: a held, coiled loading pose gathering power (body wound back, weapon drawn far back) for the first frames, then an explosive release strike at FULL extension committing the whole body, then recovery. Hold the charge, then unleash the single most extreme pose."},
	{"spin-attack", "Spin Attack", "Combat", "a spinning 360 attack", 6, 14, false, "Spin attack: rotate the whole body a full turn while sweeping the weapon around in a wide circle at full extension, then settle facing forward. Show distinct, dynamic rotation angles per frame with the weapon fully swept out."},
	{"guard-break", "Guard Break", "Combat", "stagger backward, guard smashed open", 4, 12, false, "Guard break: the raised guard is smashed violently open, the arms fly apart, the body rocks hard backward off balance, briefly exposed. The recoil reads as the defense violently failing."},
	{"counter", "Counter", "Combat", "absorb a hit, then a sharp counterattack", 5, 14, false, "Counter: a tight defensive flinch, then an immediate sharp counterattack exploding forward at FULL extension, then recovery. A crisp two-beat — defense snapping into a committed offensive strike."},
	{"taunt", "Taunt", "Combat", "a confident taunting gesture", 4, 8, true, "Taunt loop: a bold, confident provoking gesture — beckoning with a hand, chest puffed out, head cocked with attitude — looping with swagger. Feet planted, the upper body big and expressive."},
	{"draw-weapon", "Draw Weapon", "Combat", "draw a weapon into a ready stance", 5, 10, false, "Draw weapon: reach for the weapon, pull it free in a sweeping dynamic motion, then settle into a strong ready combat stance. Final frame is the confident ready pose with the weapon up."},

	// ── Magic & Skills ──
	{"cast", "Cast", "Magic & Skills", "a dramatic spell cast: gather power, then a bold forward casting thrust", 5, 12, false, "Dramatic spell cast with exaggerated, dynamic poses: a deep coiled gathering pose drawing power inward with visible tension, then an explosive forward casting thrust at FULL arm extension with a strong braced stance and the whole body committing forward, then recovery. Make the release frame the most extreme, dynamic pose. Pose changes only — any effect must be opaque, hard-edged and fused to the hands, never floating particles."},
	{"cast-fire", "Fire Cast", "Magic & Skills", "cast a fire spell with a forward blast", 6, 12, false, "Fire spell cast with dynamic, committed poses: gather energy at the hands in a deep coiled stance, then thrust both hands forward releasing the blast at full extension with the whole body driving forward. Any flame must be opaque, hard-edged, and touching the hands, not floating particles."},
	{"cast-ice", "Ice Cast", "Magic & Skills", "cast an ice spell, controlled and sharp", 6, 10, false, "Ice spell cast: a slow controlled gathering pose with the hands sweeping inward, then a sharp, pointed release thrust forward at full extension. Cold, precise and deliberate — but commit the release pose fully and dynamically."},
	{"cast-lightning", "Lightning Cast", "Magic & Skills", "cast a lightning spell, fast and sharp", 5, 14, false, "Lightning cast: a fast raise of the arm overhead charging with tension, then a sharp, snappy downward or forward strike releasing the bolt at full extension. Quick and explosive. Effects hard-edged and touching the hand only."},
	{"cast-heal", "Heal Cast", "Magic & Skills", "cast a gentle healing spell on self", 5, 8, false, "Healing cast: bring the hands together at the chest in a gentle gathering pose, then open them outward and upward in a soft, flowing release with the head tilted up. Calm and graceful, with a clear open-up gesture at the peak."},
	{"summon", "Summon", "Magic & Skills", "summon by raising both arms high", 5, 10, false, "Summon: crouch and gather low with coiled power, then rise sweeping both arms grandly upward and outward in a big calling gesture, finishing tall with the arms raised high. Build to a dramatic peak."},
	{"channel", "Channel", "Magic & Skills", "channel energy continuously", 4, 8, true, "Channeling loop: a sustained, focused pose with the hands held out gathering energy, the body tense with small pulsing shifts and a slight glow fused at the hands. Looping, intense concentration."},
	{"buff", "Buff", "Magic & Skills", "a self power-up buff gesture", 4, 10, false, "Buff cast: clench the fists and pull them inward to the body in a strong powering-up motion, the body tensing and rising, finishing in a braced, powered-up pose. Commit the final pose with energy."},
	{"shield-up", "Shield Up", "Magic & Skills", "raise a magical shield barrier", 4, 10, false, "Shield up: sweep one arm forward and out to project a barrier with a braced, committed stance behind it, then hold firm. Any barrier shape must be opaque and hard-edged, fused to the arm — never floating particles."},
	{"teleport", "Teleport", "Magic & Skills", "vanish and reappear", 5, 14, false, "Teleport: the body compresses and distorts, shrinking away to nothing in the first frames, then reforms and expands back into a solid, dynamic pose. Use silhouette compression and stretch, not particle clouds."},
	{"transform", "Transform", "Magic & Skills", "a dramatic transformation", 6, 10, false, "Transformation: a crouched, coiled gathering pose, the body tensing and shaking with building power, then bursting upward into a bold new powered-up stance. Build the tension, then release into the most dramatic final pose."},
	{"power-up", "Power Up", "Magic & Skills", "powering up with surging energy", 5, 10, true, "Power-up loop: a braced wide stance, fists clenched, the body trembling with effort and energy surging upward, hair and clothes lifting. Looping intensity at full strain; feet stay planted."},
	{"meditate", "Meditate", "Magic & Skills", "sitting meditation, calm breathing", 3, 4, true, "Meditation loop: seated cross-legged, hands resting on the knees, eyes closed, very slow calm breathing rising and falling. Minimal, serene motion; loops smoothly."},
	{"explode", "Explode", "Magic & Skills", "release an explosive burst outward", 5, 16, false, "Explosive release: gather tightly inward with coiled tension, then throw the whole body open releasing a burst outward at full extension, then settle. Use the body opening up dramatically to imply the blast; effects hard-edged and fused to the body, never floating."},

	// ── Damage & Status ──
	{"hurt", "Hurt", "Damage & Status", "recoil hard from being hit", 3, 10, false, "Hit reaction with sharp recoil: the body snaps backward from the impact, the head whips back, a brief stagger with the arms flailing, then a weakened guard pose. Make the recoil read sharply; feet roughly in place."},
	{"hurt-heavy", "Heavy Hurt", "Damage & Status", "stagger violently from a heavy hit", 4, 10, false, "Heavy hit reaction, big and violent: the whole body is thrown backward and folds from the impact, the head whipping hard back, nearly losing balance, then a struggling recovery. Much bigger and more dramatic than a normal hurt."},
	{"knockback", "Knockback", "Damage & Status", "knocked flying backward through the air", 4, 12, false, "Knockback: launched backward off the feet by a blow, the body airborne and tumbling backward dramatically, then a hard skidding stop. Show clear, exaggerated backward travel through the air."},
	{"knockdown", "Knockdown", "Damage & Status", "knocked down hard to the ground", 4, 10, false, "Knockdown: struck and losing footing, the body rotates and drops hard, landing flat on the back or side on the ground. Final frame fully down — make the fall read dramatically."},
	{"get-up", "Get Up", "Damage & Status", "push up off the ground to standing", 5, 8, false, "Get up: from lying on the ground, push up with the arms, draw the legs under, and rise through a crouch back to a strong standing stance. Clear, determined upward progression."},
	{"stun", "Stun", "Damage & Status", "stunned and wobbling in place", 4, 8, true, "Stunned loop: a dazed, slumped posture, the head lolling, the body swaying off balance, the knees buckling. Looping woozy wobble; the feet barely hold."},
	{"dizzy", "Dizzy", "Damage & Status", "dizzy, head spinning", 4, 8, true, "Dizzy loop: the head rolling in circles, the body wobbling, the arms loose and drifting for balance, unfocused. Looping disorientation. No floating star particles."},
	{"frozen", "Frozen", "Damage & Status", "frozen stiff and trembling", 3, 6, true, "Frozen loop: the body locked rigid mid-pose, the arms clamped to the sides, only a tiny brittle tremble. Stiff and immobile, shivering slightly; loops."},
	{"burning", "Burning", "Damage & Status", "on fire, flinching from the flames", 4, 12, true, "Burning loop: flinching and writhing in pain, patting at the body, hopping in discomfort. Make the panic read. Any flame must be opaque, hard-edged, and touching the body, not floating particles."},
	{"poisoned", "Poisoned", "Damage & Status", "sickened by poison, hunched", 4, 6, true, "Poisoned loop: hunched and queasy, clutching the stomach, swaying weakly, the head drooping. Looping sickly weakness; keep it clearly miserable but low-energy."},
	{"stagger", "Stagger", "Damage & Status", "stumble and barely keep balance", 4, 10, false, "Stagger: lurch off balance, the arms windmilling wildly to recover, the feet shuffling to catch the body, then steadying. Reads as almost falling — exaggerate the off-balance lurch."},
	{"death", "Death", "Damage & Status", "stagger, collapse, lie flat on the ground", 5, 8, false, "Defeat sequence: stagger, collapse to the knees, fall further down, and finally lie flat on the ground. Make the collapse dramatic; final frame clearly lying down."},
	{"death-fall", "Falling Death", "Damage & Status", "fall backward and collapse", 4, 8, false, "Falling death: thrown backward, the arms flung out wide, the body arcing back and dropping, landing flat and motionless. Dramatic backward arc; final frame fully down and still."},
	{"revive", "Revive", "Damage & Status", "rise back to life from the ground", 6, 8, false, "Revive: from lying flat, the body stirs, lifts, and rises through a kneeling pose back to a strong standing stance, the head lifting last. A clear, building return of strength."},
	{"low-hp", "Low HP", "Damage & Status", "near death, weak and hunched", 4, 6, true, "Low-HP loop: hunched and exhausted, one hand braced on a knee, heavy labored breathing, a slight unsteady sway. Barely standing; looping fatigue, clearly drained but readable."},
	{"defeat", "Defeat", "Damage & Status", "drop to the knees in defeat", 4, 8, false, "Defeat: the shoulders sag, the body sinks down onto the knees, the head bowing low in surrender. Ends kneeling and dejected — make the slump read heavily."},

	// ── Emotion ──
	{"wave", "Wave", "Emotion", "a friendly hand wave, body still", 4, 8, true, "Friendly greeting: one arm raises and waves clearly side to side across the frames while the rest of the body stays still. The hand in clearly different positions each frame, big and readable. Feet planted; loops."},
	{"cheer", "Cheer", "Emotion", "cheer with both arms thrown up", 4, 10, true, "Cheering loop: throw both arms up overhead repeatedly with a lively hop or bounce, head up, joyful and energetic. Big, exuberant looping celebration."},
	{"clap", "Clap", "Emotion", "clapping hands", 4, 10, true, "Clapping loop: bring both hands together and apart in front of the chest repeatedly with a slight body bounce. Hands clearly wide open then together across the frames; loops."},
	{"bow", "Bow", "Emotion", "a deep respectful bow", 4, 8, false, "Bow: from standing, bend forward at the waist into a deep respectful bow, hold briefly, then rise back up. Show the full, committed forward bend."},
	{"nod", "Nod", "Emotion", "nodding the head yes", 3, 8, false, "Nod: tip the head clearly down and back up in agreement, with a small body settle. The head moves distinctly down then up; the body otherwise still."},
	{"shake-head", "Shake Head", "Emotion", "shaking the head no", 4, 8, false, "Head shake: turn the head clearly left and right in refusal, the shoulders slightly tense. The head rotates distinctly side to side; the body otherwise still."},
	{"laugh", "Laugh", "Emotion", "laughing happily", 4, 8, true, "Laughing loop: head tipped back, shoulders bouncing hard with laughter, maybe a hand to the belly, a big open smile. A lively looping bounce of joy."},
	{"cry", "Cry", "Emotion", "crying sadly", 4, 6, true, "Crying loop: hands toward the face, the shoulders shaking with sobs, the head bowed, the body hunched. Looping sad tremble. Tears optional but must be small and on the face."},
	{"angry", "Angry", "Emotion", "furious, fists clenched", 4, 8, true, "Angry loop: fists clenched hard, shoulders raised and tense, the body trembling with rage and leaning forward, brows down. Looping fury; feet planted, make the tension read."},
	{"surprised", "Surprised", "Emotion", "startled and recoiling", 3, 12, false, "Surprise: a sharp startled jolt — the body snaps upright and back, the arms fly up, the head rears, eyes wide. A quick exaggerated recoil, then a frozen shocked pose."},
	{"think", "Think", "Emotion", "thinking with a hand on the chin", 3, 6, true, "Thinking loop: one hand to the chin, the head tilted, weight shifting slowly side to side, an occasional small head tilt. Pondering, subtle looping motion."},
	{"point", "Point", "Emotion", "point forward decisively", 4, 10, false, "Pointing: draw the arm back, then thrust it forward extending one finger to point decisively at FULL extension, the body leaning into it, then hold. Peak frame fully extended forward."},
	{"salute", "Salute", "Emotion", "a crisp military salute", 4, 8, false, "Salute: snap one hand up to the brow in a crisp military salute, the body straightening sharply to attention, hold, then lower. Sharp, formal and precise."},
	{"dance", "Dance", "Emotion", "rhythmic, lively dancing", 6, 10, true, "Dancing loop: rhythmic full-body movement — hips and arms swaying, weight shifting foot to foot, the head bobbing to a beat. Distinct, fun, expressive poses per frame; loops smoothly."},
	{"victory", "Victory", "Emotion", "a triumphant victory pose", 4, 8, true, "Victory loop: a triumphant pose — fist pumped or arms thrown up, chest out, a confident bounce. Big, proud, energetic looping celebration."},
	{"sad", "Sad", "Emotion", "sad and downcast", 4, 4, true, "Sad loop: shoulders slumped, head down, arms hanging limp, a slow heavy sway and sigh. Looping melancholy; minimal, weighty motion."},
	{"scared", "Scared", "Emotion", "frightened and cowering", 4, 8, true, "Scared loop: cowering back, the arms raised defensively in front of the face, the body trembling and shrinking, the knees together. Looping fear; make the cower read clearly."},
	{"yawn", "Yawn", "Emotion", "a big sleepy yawn and stretch", 4, 6, false, "Yawn: a big stretch with the arms rising and the head tilting back, the mouth wide in a yawn, then the arms lower and the shoulders settle. One clear, full stretch-and-relax."},

	// ── Interaction ──
	{"pick-up", "Pick Up", "Interaction", "bend down and pick up an item", 4, 10, false, "Pick up: bend deeply at the knees and waist down toward the ground, close the hand as if grasping an item, then rise back up holding it. Clear, committed down-then-up motion."},
	{"carry", "Carry", "Interaction", "walk while carrying a heavy load", 6, 8, true, "Carrying walk loop: a walking cycle with both arms held forward or up bearing a load, leaning back to balance the weight, with shorter strained steps. Looping burdened walk; the effort reads."},
	{"push", "Push", "Interaction", "push a heavy object forward", 6, 8, true, "Pushing loop: leaning hard forward with both arms extended against an object, the legs driving with alternating straining steps. Looping effortful push; exaggerate the strain and forward lean."},
	{"pull", "Pull", "Interaction", "pull a heavy object backward", 6, 8, true, "Pulling loop: leaning hard back with both arms drawn in gripping something, the legs stepping backward and digging in, straining. Looping effortful pull; exaggerate the strain and back lean."},
	{"open", "Open", "Interaction", "open a door or chest", 4, 10, false, "Open: reach forward toward a handle, grip and pull or push it open with a clear turning motion, leaning in. A readable reach-and-open action with the arm doing the work."},
	{"eat", "Eat", "Interaction", "eating food", 4, 8, false, "Eating: raise a hand to the mouth as if holding food, take a bite with a small head tilt, lower the hand, chew. Clear, readable hand-to-mouth motion."},
	{"drink", "Drink", "Interaction", "drinking from a cup", 4, 8, false, "Drinking: raise a hand to the mouth as if holding a cup, tip the head back to drink, then lower. Clear, readable raise-tip-lower motion."},
	{"read", "Read", "Interaction", "reading a held book", 4, 6, true, "Reading loop: both hands held out front as if holding an open book, the head tilted down scanning, an occasional small head shift or page turn. Calm, readable looping study."},
	{"dig", "Dig", "Interaction", "digging with a shovel", 6, 8, true, "Digging loop: thrust a shovel down into the ground, scoop, then lift and toss the dirt aside with effort, and return. Looping dig cycle with clear, committed down-scoop-toss phases."},
	{"mine", "Mine", "Interaction", "swinging a pickaxe to mine", 6, 10, true, "Mining loop: raise a pickaxe high overhead, swing it down hard into rock with a strong impact recoil, then lift back up. Looping swing cycle with a clear, forceful strike frame."},
	{"chop", "Chop", "Interaction", "chopping with an axe", 6, 10, true, "Chopping loop: raise an axe up and back, swing it down hard into a target with an impact jolt, then recover up. Looping chop cycle with a clear, forceful strike frame."},
	{"fish", "Fish", "Interaction", "fishing, cast and wait", 5, 6, true, "Fishing loop: holding a rod out front, a slow gentle bob of the line and a small body sway while waiting, an occasional tiny tug check. Calm, patient looping wait."},
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
