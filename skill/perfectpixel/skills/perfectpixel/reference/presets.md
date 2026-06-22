# PerfectPixel motion preset catalog

The list of English state keys used with `-states`. Generated from the same single
source as the `presets` in `ppgen -dump`. Each entry: `key` (label) — default frame count
/ fps / loop behavior.

## Basic motions

- `idle` (idle) — 4 frames / 6fps / loop
- `idle-combat` (combat idle) — 4 frames / 8fps / loop
- `walk` (walk) — 6 frames / 10fps / loop
- `run` (run) — 6 frames / 12fps / loop
- `sprint` (sprint) — 6 frames / 14fps / loop
- `jump` (jump) — 5 frames / 10fps / one-shot
- `fall` (fall) — 4 frames / 10fps / loop
- `land` (land) — 4 frames / 12fps / one-shot
- `crouch` (crouch) — 4 frames / 8fps / one-shot
- `crawl` (crawl) — 6 frames / 8fps / loop
- `climb` (climb) — 6 frames / 8fps / loop
- `swim` (swim) — 6 frames / 8fps / loop
- `dash` (dash) — 4 frames / 14fps / one-shot
- `roll` (roll) — 5 frames / 14fps / one-shot
- `slide` (slide) — 4 frames / 12fps / one-shot
- `sit` (sit) — 4 frames / 8fps / one-shot
- `sleep` (sleep) — 4 frames / 4fps / loop
- `turn` (turn around) — 4 frames / 10fps / one-shot

## Combat

- `attack` (attack) — 5 frames / 12fps / one-shot
- `attack-heavy` (heavy attack) — 6 frames / 10fps / one-shot
- `combo` (combo attack) — 6 frames / 14fps / one-shot
- `slash` (slash) — 5 frames / 14fps / one-shot
- `stab` (stab) — 4 frames / 14fps / one-shot
- `punch` (punch) — 4 frames / 14fps / one-shot
- `kick` (kick) — 5 frames / 14fps / one-shot
- `uppercut` (uppercut) — 4 frames / 14fps / one-shot
- `block` (block) — 3 frames / 10fps / loop
- `parry` (parry) — 4 frames / 16fps / one-shot
- `dodge` (dodge) — 4 frames / 16fps / one-shot
- `backstep` (backstep) — 4 frames / 14fps / one-shot
- `shoot` (shoot) — 4 frames / 14fps / one-shot
- `reload` (reload) — 5 frames / 10fps / one-shot
- `aim` (aim) — 3 frames / 10fps / loop
- `throw` (throw) — 5 frames / 12fps / one-shot
- `charge-attack` (charge attack) — 6 frames / 12fps / one-shot
- `spin-attack` (spin attack) — 6 frames / 14fps / one-shot
- `guard-break` (guard break) — 4 frames / 12fps / one-shot
- `counter` (counter) — 5 frames / 14fps / one-shot
- `taunt` (taunt) — 4 frames / 8fps / loop
- `draw-weapon` (draw weapon) — 5 frames / 10fps / one-shot

## Magic / skills

- `cast` (cast) — 5 frames / 12fps / one-shot
- `cast-fire` (cast fire) — 6 frames / 12fps / one-shot
- `cast-ice` (cast ice) — 6 frames / 10fps / one-shot
- `cast-lightning` (cast lightning) — 5 frames / 14fps / one-shot
- `cast-heal` (cast heal) — 5 frames / 8fps / one-shot
- `summon` (summon) — 5 frames / 10fps / one-shot
- `channel` (channel) — 4 frames / 8fps / loop
- `buff` (buff) — 4 frames / 10fps / one-shot
- `shield-up` (shield up) — 4 frames / 10fps / one-shot
- `teleport` (teleport) — 5 frames / 14fps / one-shot
- `transform` (transform) — 6 frames / 10fps / one-shot
- `power-up` (power up) — 5 frames / 10fps / loop
- `meditate` (meditate) — 4 frames / 4fps / loop
- `explode` (explode) — 5 frames / 16fps / one-shot

## Damage / status effects

- `hurt` (hurt) — 3 frames / 10fps / one-shot
- `hurt-heavy` (heavy hurt) — 4 frames / 10fps / one-shot
- `knockback` (knockback) — 4 frames / 12fps / one-shot
- `knockdown` (knockdown) — 4 frames / 10fps / one-shot
- `get-up` (get up) — 5 frames / 8fps / one-shot
- `stun` (stun) — 4 frames / 8fps / loop
- `dizzy` (dizzy) — 4 frames / 8fps / loop
- `frozen` (frozen) — 3 frames / 6fps / loop
- `burning` (burning) — 4 frames / 12fps / loop
- `poisoned` (poisoned) — 4 frames / 6fps / loop
- `stagger` (stagger) — 4 frames / 10fps / one-shot
- `death` (death) — 5 frames / 8fps / one-shot
- `death-fall` (fall to death) — 4 frames / 8fps / one-shot
- `revive` (revive) — 6 frames / 8fps / one-shot
- `low-hp` (near death) — 4 frames / 6fps / loop
- `defeat` (defeat) — 4 frames / 8fps / one-shot

## Emotion / expression

- `wave` (wave) — 4 frames / 8fps / loop
- `cheer` (cheer) — 4 frames / 10fps / loop
- `clap` (clap) — 4 frames / 10fps / loop
- `bow` (bow) — 4 frames / 8fps / one-shot
- `nod` (nod) — 3 frames / 8fps / one-shot
- `shake-head` (shake head) — 4 frames / 8fps / one-shot
- `laugh` (laugh) — 4 frames / 8fps / loop
- `cry` (cry) — 4 frames / 6fps / loop
- `angry` (angry) — 4 frames / 8fps / loop
- `surprised` (surprised) — 3 frames / 12fps / one-shot
- `think` (think) — 4 frames / 6fps / loop
- `point` (point) — 4 frames / 10fps / one-shot
- `salute` (salute) — 4 frames / 8fps / one-shot
- `dance` (dance) — 6 frames / 10fps / loop
- `victory` (victory) — 4 frames / 8fps / loop
- `sad` (sad) — 4 frames / 4fps / loop
- `scared` (scared) — 4 frames / 8fps / loop
- `yawn` (yawn) — 4 frames / 6fps / one-shot

## Interaction

- `pick-up` (pick up) — 4 frames / 10fps / one-shot
- `carry` (carry) — 6 frames / 8fps / loop
- `push` (push) — 6 frames / 8fps / loop
- `pull` (pull) — 6 frames / 8fps / loop
- `open` (open) — 4 frames / 10fps / one-shot
- `eat` (eat) — 4 frames / 8fps / one-shot
- `drink` (drink) — 4 frames / 8fps / one-shot
- `read` (read) — 4 frames / 6fps / loop
- `dig` (dig) — 6 frames / 8fps / loop
- `mine` (mine) — 6 frames / 10fps / loop
- `chop` (chop) — 6 frames / 10fps / loop
- `fish` (fish) — 5 frames / 6fps / loop
