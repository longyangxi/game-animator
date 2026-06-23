// Frontend domain type definitions

export interface FrameTransform {
  scale: number; // relative to the frame content; clamped to [0.5, 1.5]
  dx: number;    // cell-pixel horizontal offset
  dy: number;    // cell-pixel vertical offset
}

export interface FrameItem {
  id: string;
  png: string; // dataURL — source frame, never mutated
  selected: boolean;
  transform?: FrameTransform; // undefined == identity
}

export interface FrameScores {
  identity: number;
  motion: number;
  contact: number;
  overall: number;
}

export type StateStatus = "idle" | "generating" | "done" | "error";

export interface StateDef {
  id: string;
  name: string; // English state name (for export)
  label: string; // display name
  frames: number;
  fps: number;
  loop: boolean;
  action: string;
  status: StateStatus;
  error?: string;
  rawStrip?: string;
  items: FrameItem[];
  warnings: string[];
  feedback: string;
  scores?: FrameScores;
  facing?: string; // 8-direction key (south, etc.; no direction instruction when unset)
  dirBase?: string; // base state name of the 8-direction set (only when part of a set)
  mirrorOf?: string; // mirror source direction key (west→east, etc.; not AI-generated)
}

// Same structure as backend sprite.DirectionInfo (ListDirections response)
export interface DirectionInfo {
  key: string;
  label: string;
  short: string;
  mirrorOf: string;
  row: number;
  col: number;
}

// Direction key → label (returns the key as-is if not in the list)
export function directionLabel(directions: DirectionInfo[], key: string | undefined): string {
  if (!key) return "";
  return directions.find((d) => d.key === key)?.label ?? key;
}

export interface CharacterDef {
  image: string | null; // dataURL
  name: string; // used as the export file prefix
  description: string;
  styleKey: string;
  styleCustom: string;
  perspective: "flat" | "iso"; // 2D (default) or 2.5D isometric
}

export interface StatePreset {
  name: string;
  label: string;
  frames: number;
  fps: number;
  loop: boolean;
  action: string;
}

// Same structure as backend sprite.PresetInfo (ListPresets response)
export interface PresetInfo {
  name: string;
  label: string;
  category: string;
  action: string;
  frames: number;
  fps: number;
  loop: boolean;
}

export const STATE_PRESETS: StatePreset[] = [
  { name: "idle", label: "Idle", frames: 4, fps: 6, loop: true, action: "subtle breathing idle in place" },
  { name: "walk", label: "Walk", frames: 6, fps: 10, loop: true, action: "side-view walking cycle facing right" },
  { name: "run", label: "Run", frames: 6, fps: 12, loop: true, action: "fast side-view running cycle facing right" },
  { name: "jump", label: "Jump", frames: 5, fps: 10, loop: false, action: "crouch, take off, airborne peak, land" },
  { name: "attack", label: "Attack", frames: 5, fps: 12, loop: false, action: "melee attack with wind-up, strike, recovery" },
  { name: "hurt", label: "Hurt", frames: 3, fps: 10, loop: false, action: "recoil from being hit" },
  { name: "death", label: "Death", frames: 5, fps: 8, loop: false, action: "stagger, collapse, lie flat on the ground" },
  { name: "wave", label: "Wave", frames: 4, fps: 8, loop: true, action: "friendly hand wave, body still" },
];

export const STYLE_OPTIONS = [
  { key: "pixel", label: "Pixel Art" },
  { key: "chibi", label: "Chibi" },
  { key: "cartoon", label: "Cartoon" },
  { key: "retro16", label: "16-bit Retro" },
  { key: "custom", label: "Custom" },
];

export const CELL_SIZES = [128, 256, 512];

let seq = 0;
export function uid(prefix: string): string {
  seq += 1;
  return `${prefix}-${Date.now().toString(36)}-${seq}`;
}

export function presetToState(p: StatePreset): StateDef {
  return {
    id: uid("st"),
    name: p.name,
    label: p.label,
    frames: p.frames,
    fps: p.fps,
    loop: p.loop,
    action: p.action,
    status: "idle",
    items: [],
    warnings: [],
    feedback: "",
  };
}

// Converts a ListPresets catalog entry into an animation state.
export function presetInfoToState(p: PresetInfo): StateDef {
  return {
    id: uid("st"),
    name: p.name,
    label: p.label,
    frames: p.frames,
    fps: p.fps,
    loop: p.loop,
    action: p.action,
    status: "idle",
    items: [],
    warnings: [],
    feedback: "",
  };
}

// Fallback catalog derived from STATE_PRESETS (used when ListPresets fails)
export const FALLBACK_PRESETS: PresetInfo[] = STATE_PRESETS.map((p) => ({
  ...p,
  category: "Basics",
}));

export function selectedFrames(s: StateDef): FrameItem[] {
  return s.items.filter((f) => f.selected);
}
