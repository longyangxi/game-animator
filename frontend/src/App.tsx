import { useEffect, useRef, useState } from "react";
import { Images, Package, Plus, Scissors, Settings, X } from "lucide-react";
import { CancelGeneration, ClearSession, ExportProject, ExportRawStrips, GenerateState, GetSettings, ListDirections, ListPresets, LoadSession, MirrorFrames, RevealInFinder, SaveSession } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import CharacterPanel from "./components/CharacterPanel";
import GalleryModal from "./components/GalleryModal";
import PreviewPanel from "./components/PreviewPanel";
import SettingsModal, { ISettings } from "./components/SettingsModal";
import StatesPanel from "./components/StatesPanel";
import { Button } from "./components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./components/ui/dialog";
import { CharacterDef, DirectionInfo, FALLBACK_PRESETS, FrameItem, PresetInfo, StateDef, selectedFrames, uid } from "./types";
import { useI18n } from "./i18n";
import { bakeTransformed, framePad } from "./lib/frameTransform";
import { directionName } from "./i18n/catalog";
import logoUrl from "./assets/logo.svg";

interface IToast {
  id: string;
  kind: "info" | "error" | "success";
  text: string;
}

// Check whether the active provider has a key
const hasActiveKey = (s: ISettings | null) => !!s?.providers?.[s.provider]?.hasKey;

const PROVIDER_LABELS: Record<string, string> = {
  gemini: "Gemini",
  openai: "OpenAI",
  openrouter: "OpenRouter",
  fal: "fal.ai",
  byteplus: "BytePlus",
  replicate: "Replicate",
};

export default function App() {
  const { t, lang } = useI18n();
  const [settings, setSettings] = useState<ISettings | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showGallery, setShowGallery] = useState(false);
  const [confirmNew, setConfirmNew] = useState(false);
  const [character, setCharacter] = useState<CharacterDef>({
    image: null,
    name: "",
    description: "",
    styleKey: "pixel",
    styleCustom: "",
    perspective: "flat",
  });
  const [cellSize, setCellSize] = useState(256);
  const [padFrac, setPadFrac] = useState(0); // working margin per side (fraction of cell); 0 = no margin
  const [states, setStates] = useState<StateDef[]>([]);
  const [directions, setDirections] = useState<DirectionInfo[]>([]);
  const [presets, setPresets] = useState<PresetInfo[]>(FALLBACK_PRESETS);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState("");
  const [toasts, setToasts] = useState<IToast[]>([]);

  // References to the latest state (used in async loops)
  const statesRef = useRef(states);
  statesRef.current = states;
  const charRef = useRef(character);
  charRef.current = character;
  const cellRef = useRef(cellSize);
  cellRef.current = cellSize;
  const padRef = useRef(padFrac);
  padRef.current = padFrac;
  const cancelRef = useRef(false); // Flag to abort the entire generation loop
  const busyRef = useRef(busy);
  busyRef.current = busy;

  const restoredRef = useRef(false); // Prevent auto-save before session restoration completes

  useEffect(() => {
    refreshSettings();
    ListDirections()
      .then((list: any) => setDirections(list ?? []))
      .catch(() => {});
    ListPresets()
      .then((list: any) => {
        if (Array.isArray(list) && list.length > 0) setPresets(list);
      })
      .catch(() => {}); // Keep FALLBACK_PRESETS on failure
    const off = EventsOn("progress", (data: any) => {
      const st = data?.state ? `[${data.state}] ` : "";
      setProgress(`${st}${data?.message ?? ""}`);
    });

    // Restore the previous work session
    (async () => {
      try {
        const raw = await LoadSession();
        if (raw) {
          const s = JSON.parse(raw);
          if (s?.character) {
            setCharacter({
              image: s.character.image ?? null,
              name: s.character.name ?? "",
              description: s.character.description ?? "",
              styleKey: s.character.styleKey ?? "pixel",
              styleCustom: s.character.styleCustom ?? "",
              perspective: s.character.perspective ?? "flat",
            });
          }
          if (typeof s?.cellSize === "number") setCellSize(s.cellSize);
          if (typeof s?.padFrac === "number") setPadFrac(s.padFrac);
          if (Array.isArray(s?.states)) {
            // Safely clean up states left mid-generation; exclude legacy procedural animation states
            const states: StateDef[] = s.states
              .filter((st: any) => st?.mode !== "procedural")
              .map((st: StateDef) => ({
                ...st,
                status: st.status === "generating" ? (st.items?.length > 0 ? "done" : "idle") : st.status,
              }));
            setStates(states);
            if (s.selectedId && states.some((x) => x.id === s.selectedId)) setSelectedId(s.selectedId);
          }
        }
      } catch {
        // Ignore corrupted sessions
      } finally {
        restoredRef.current = true;
      }
    })();

    return off;
  }, []);

  // Auto-save the work session (debounced)
  useEffect(() => {
    if (!restoredRef.current) return;
    const t = setTimeout(() => {
      SaveSession(JSON.stringify({ v: 1, character, cellSize, padFrac, states, selectedId })).catch(() => {});
    }, 1200);
    return () => clearTimeout(t);
  }, [character, cellSize, padFrac, states, selectedId]);

  // Global shortcuts: ⌘, Settings / ⌘E Export / ⌘G Gallery
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      if (e.key === ",") {
        e.preventDefault();
        setShowSettings(true);
      } else if (e.key.toLowerCase() === "e") {
        e.preventDefault();
        if (!busyRef.current) handleExport();
      } else if (e.key.toLowerCase() === "g") {
        e.preventDefault();
        setShowGallery((v) => !v);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const refreshSettings = async (): Promise<ISettings | null> => {
    try {
      const s = (await GetSettings()) as unknown as ISettings;
      setSettings(s);
      if (!hasActiveKey(s)) setShowSettings(true);
      return s;
    } catch {
      return null;
    }
  };

  const toast = (kind: IToast["kind"], text: string) => {
    const t: IToast = { id: uid("toast"), kind, text };
    setToasts((prev) => [...prev, t]);
    setTimeout(() => setToasts((prev) => prev.filter((x) => x.id !== t.id)), 5000);
  };

  const updateState = (id: string, patch: Partial<StateDef>) => {
    // Update statesRef immediately: the sequential direction-set generation loop must
    // be able to read the previous direction's results (front strip, mirror source) before re-render
    statesRef.current = statesRef.current.map((s) => (s.id === id ? { ...s, ...patch } : s));
    setStates(statesRef.current);
  };

  const generateOne = async (id: string, feedback = ""): Promise<boolean> => {
    const st = statesRef.current.find((s) => s.id === id);
    const ch = charRef.current;
    if (!st || !ch.image) return false;

    const prevItems = st.items;
    updateState(id, { status: "generating", error: undefined, warnings: [], feedback });
    try {
      // Mirror direction: horizontally flip the source direction's frames without an AI call
      if (st.mirrorOf) {
        const src = statesRef.current.find(
          (s) => s.dirBase === st.dirBase && s.facing === st.mirrorOf && s.status === "done" && s.items.length > 0
        );
        if (!src) {
          updateState(id, {
            status: "error",
            items: prevItems,
            error: t("err_mirror_source", { dir: directionName(st.mirrorOf, lang) }),
          });
          return false;
        }
        const mirrored: string[] = (await MirrorFrames(src.items.map((f) => f.png))) ?? [];
        const items: FrameItem[] = mirrored.map((png, i) => ({
          id: uid("fr"),
          png,
          selected: src.items[i]?.selected ?? true,
        }));
        updateState(id, {
          status: items.length > 0 ? "done" : "error",
          error: items.length > 0 ? undefined : t("err_mirror_fail"),
          items,
          warnings: [],
          scores: src.scores ? { ...src.scores } : undefined,
        });
        return items.length > 0;
      }

      // If part of a direction set, pass the front (south) strip as a motion reference
      let refStrip = "";
      if (st.dirBase && st.facing && st.facing !== "south") {
        const south = statesRef.current.find((s) => s.dirBase === st.dirBase && s.facing === "south");
        refStrip = south?.rawStrip ?? "";
      }

      const res: any = await GenerateState({
        baseImage: ch.image,
        description: ch.description,
        styleKey: ch.styleKey,
        styleCustom: ch.styleCustom,
        perspective: ch.perspective,
        cellSize: cellRef.current,
        safeMargin: 0,
        feedback,
        refStrip,
        state: { name: st.name, frames: st.frames, fps: st.fps, loop: st.loop, action: st.action, facing: st.facing ?? "" },
      } as any);

      const items: FrameItem[] = (res.frames ?? []).map((png: string) => ({
        id: uid("fr"),
        png,
        selected: true,
      }));
      const scores = res.scores
        ? {
            identity: (res.scores as any).identity ?? 0,
            motion: (res.scores as any).motion ?? 0,
            contact: (res.scores as any).contact ?? 0,
            overall: (res.scores as any).overall ?? 0,
          }
        : undefined;
      // If the quality score is very low, add an explicit warning (so the user can consider regenerating)
      const extraWarnings: string[] = [];
      if (scores && scores.overall < 0.35) {
        extraWarnings.push(t("score_low_warn"));
      }
      updateState(id, {
        status: items.length > 0 ? "done" : "error",
        error: items.length > 0 ? undefined : t("err_no_frames"),
        items,
        rawStrip: res.rawStrip || undefined,
        warnings: [...(res.warnings ?? []), ...extraWarnings],
        scores,
      });
      return items.length > 0;
    } catch (e) {
      const msg = String(e);
      if (msg.includes("canceled")) {
        // Canceled: preserve the previous results and return quietly
        updateState(id, { status: prevItems.length > 0 ? "done" : "idle", items: prevItems });
        toast("info", t("toast_gen_canceled"));
        return false;
      }
      updateState(id, { status: "error", error: msg });
      return false;
    }
  };

  // Generate multiple states in parallel under a concurrency limit and return the success count.
  // (Guards against rate limits on some providers like fal — default of 3, same as the 8-direction set)
  const GEN_CONCURRENCY = 3;
  const generateBatch = async (ids: string[]): Promise<number> => {
    const queue = [...ids];
    let ok = 0;
    const worker = async () => {
      while (queue.length > 0) {
        if (cancelRef.current) return;
        const id = queue.shift()!;
        setSelectedId(id);
        if (await generateOne(id)) ok += 1;
      }
    };
    await Promise.all(Array.from({ length: Math.min(GEN_CONCURRENCY, queue.length) }, worker));
    return ok;
  };

  const handleGenerate = async (id: string) => {
    if (busy) return;
    setBusy(true);
    cancelRef.current = false;
    setSelectedId(id);
    await generateOne(id);
    setBusy(false);
    setProgress("");
  };

  const handleRegenerate = async (id: string, feedback: string) => {
    if (busy) return;
    setBusy(true);
    cancelRef.current = false;
    await generateOne(id, feedback);
    setBusy(false);
    setProgress("");
  };

  // 8-direction set: 5 directions AI-generated (south as front reference) + 3 directions horizontally mirrored
  const handleGenerateDirectionSet = async (id: string) => {
    if (busy || directions.length === 0) return;
    const origin = statesRef.current.find((s) => s.id === id);
    if (!origin || !charRef.current.image) return;

    setBusy(true);
    cancelRef.current = false;

    const base = origin.dirBase ?? origin.name;
    const labelBase = origin.dirBase ? origin.label.split("·")[0] : origin.label;

    // Ensure set state: convert the clicked state to south and add only the missing directions
    let next = [...statesRef.current];
    if (!origin.dirBase) {
      // Existing frames generated for another direction can't be reused as south, so reset them
      const keepItems = !origin.facing || origin.facing === "south";
      next = next.map((s) =>
        s.id === id
          ? {
              ...s,
              name: `${base}-south`,
              label: `${labelBase}·Front`,
              dirBase: base,
              facing: "south",
              ...(keepItems ? {} : { items: [], status: "idle" as const, rawStrip: undefined, warnings: [] }),
            }
          : s
      );
    }
    const inSet = (key: string) => next.some((s) => s.dirBase === base && s.facing === key);
    for (const d of directions) {
      if (inSet(d.key)) continue;
      next.push({
        id: uid("st"),
        name: `${base}-${d.key}`,
        label: `${labelBase}·${d.label}`,
        frames: origin.frames,
        fps: origin.fps,
        loop: origin.loop,
        action: origin.action,
        status: "idle",
        items: [],
        warnings: [],
        feedback: "",
        facing: d.key,
        dirBase: base,
        mirrorOf: d.mirrorOf || undefined,
      });
    }
    statesRef.current = next;
    setStates(next);

    // Generation order: south first (front reference, motion reference for the other directions) →
    // the remaining AI directions in parallel under a concurrency limit → 3 mirrors (local, fast)
    setSelectedId(id); // Select south so the set preview (direction grid) is visible
    let ok = 0;
    let failed = 0;
    const genKey = async (key: string): Promise<boolean | null> => {
      const st = statesRef.current.find((s) => s.dirBase === base && s.facing === key);
      if (!st) return null;
      if (st.status === "done" && st.items.length > 0) return true; // Reuse an already-completed direction
      return generateOne(st.id);
    };
    const tally = (r: boolean | null) => {
      if (r === true) ok += 1;
      else if (r === false) failed += 1;
    };

    // 1) south (for motion reference) — must complete first
    if (!cancelRef.current) tally(await genKey("south"));

    // 2) Remaining AI directions in parallel (limited to a concurrency of 3 to guard against fal rate limits)
    const aiRest = ["east", "north", "south-east", "north-east"];
    const queue = [...aiRest];
    const CONCURRENCY = 3;
    const worker = async () => {
      while (queue.length > 0) {
        if (cancelRef.current) return;
        const key = queue.shift()!;
        tally(await genKey(key));
      }
    };
    await Promise.all(Array.from({ length: Math.min(CONCURRENCY, queue.length) }, worker));

    // 3) Mirror directions (after east/se/ne complete, horizontally flip without an AI call)
    const mirrorKeys = directions.filter((d) => d.mirrorOf).map((d) => d.key);
    for (const key of mirrorKeys) {
      if (cancelRef.current) break;
      tally(await genKey(key));
    }
    setBusy(false);
    setProgress("");
    if (!cancelRef.current) {
      toast(
        failed === 0 ? "success" : "info",
        failed === 0 ? t("toast_dirset", { ok }) : t("toast_dirset_failed", { ok, failed })
      );
    }
  };

  const handleGenerateAll = async () => {
    if (busy) return;
    setBusy(true);
    cancelRef.current = false;
    const pending = statesRef.current.filter((s) => s.status !== "done");
    // Mirror directions require their source direction to be generated first, so generate
    // non-mirror directions in parallel first, then the mirrors in parallel.
    const nonMirror = pending.filter((s) => !s.mirrorOf).map((s) => s.id);
    const mirror = pending.filter((s) => s.mirrorOf).map((s) => s.id);
    const total = nonMirror.length + mirror.length;
    let ok = 0;
    ok += await generateBatch(nonMirror);
    if (!cancelRef.current && mirror.length > 0) {
      ok += await generateBatch(mirror);
    }
    setBusy(false);
    setProgress("");
    if (total > 0 && !cancelRef.current) {
      toast(ok === total ? "success" : "info", t("toast_genall", { ok, total }));
    }
  };

  // Add N custom states at once and generate them sequentially right away
  const handleAddCustomBatch = async (count: number) => {
    if (busy) return;
    const n = Math.max(1, Math.min(10, count));
    const base = statesRef.current.length;
    const created: StateDef[] = Array.from({ length: n }, (_, i) => ({
      id: uid("st"),
      name: `custom${base + 1 + i}`,
      label: "Custom",
      frames: 4,
      fps: 8,
      loop: true,
      action: "",
      status: "idle",
      items: [],
      warnings: [],
      feedback: "",
    }));
    // Update statesRef immediately so generateOne can find the new states before re-render
    statesRef.current = [...statesRef.current, ...created];
    setStates(statesRef.current);
    const ids = created.map((s) => s.id);
    setSelectedId(ids[ids.length - 1]);

    if (!charRef.current.image || !hasActiveKey(settings)) return;

    setBusy(true);
    cancelRef.current = false;
    // Generate the entire batch in parallel under a concurrency limit
    const ok = await generateBatch(ids);
    setBusy(false);
    setProgress("");
    if (!cancelRef.current) {
      toast(ok === ids.length ? "success" : "info", t("toast_custom", { ok, total: ids.length }));
    }
  };

  const handleCancel = () => {
    cancelRef.current = true;
    CancelGeneration();
  };

  // New project: if there is existing work, show an in-app confirmation modal; otherwise reset immediately.
  // (window.confirm is not used because it doesn't work in the Wails WKWebView)
  const handleNewProject = () => {
    if (busy) return;
    const hasWork = !!charRef.current.image || statesRef.current.length > 0;
    if (hasWork) {
      setConfirmNew(true);
      return;
    }
    resetProject();
  };

  const resetProject = async () => {
    setConfirmNew(false);
    setCharacter({ image: null, name: "", description: "", styleKey: "pixel", styleCustom: "", perspective: "flat" });
    setStates([]);
    setSelectedId(null);
    setCellSize(256);
    try {
      await ClearSession();
    } catch {
      // Ignore failures to delete the session file (the next auto-save will overwrite it)
    }
    toast("info", t("toast_new_project"));
  };

  const handleExport = async () => {
    const done = statesRef.current.filter((s) => s.status === "done" && selectedFrames(s).length > 0);
    if (done.length === 0) {
      toast("error", t("toast_no_export"));
      return;
    }
    try {
      const pad = framePad(cellRef.current, padRef.current);
      const outDir: any = await ExportProject({
        character: charRef.current.name.trim() || "character",
        // Frames are baked onto the padded working canvas; the atlas cell must match.
        cellSize: cellRef.current + 2 * pad,
        states: await Promise.all(
          done.map(async (s) => ({
            name: s.name,
            fps: s.fps,
            loop: s.loop,
            frames: await Promise.all(
              selectedFrames(s).map((f) => bakeTransformed(f.png, f.transform, cellRef.current, pad)),
            ),
          })),
        ),
      } as any);
      if (outDir) {
        toast("success", t("toast_export_done", { dir: outDir }));
        RevealInFinder(outDir);
      }
    } catch (e) {
      toast("error", String(e));
    }
  };

  // Dev/testing: dump every stage state's pre-slice strip as PNG testdata for
  // offline slicing tests. Costs no generation tokens — it reuses what's on stage.
  const handleExportRawStrips = async () => {
    const withStrip = statesRef.current.filter((s) => !!s.rawStrip);
    if (withStrip.length === 0) {
      toast("error", t("toast_no_export"));
      return;
    }
    try {
      const outDir: any = await ExportRawStrips(
        withStrip.map((s) => ({ name: s.name, rawStrip: s.rawStrip as string, expected: s.frames })) as any,
      );
      if (outDir) {
        toast("success", t("toast_export_done", { dir: outDir }));
        RevealInFinder(outDir);
      }
    } catch (e) {
      toast("error", String(e));
    }
  };

  const selectedState = states.find((s) => s.id === selectedId) ?? null;
  const exportable = states.some((s) => s.status === "done" && selectedFrames(s).length > 0);
  const hasRawStrips = states.some((s) => !!s.rawStrip);

  return (
    <div className="app">
      <header className="topbar">
        <div className="logo">
          <img className="logo-mark" src={logoUrl} alt={t("logo_alt")} />
        </div>
        <div className="topbar-status">
          {progress && (
            <span className="progress-pill">
              <span className="spinner" />
              {progress}
              <button className="pp-cancel" onClick={handleCancel} title={t("cancel_generation")}>
                <X size={10} />
              </button>
            </span>
          )}
        </div>
        <div className="topbar-actions">
          <Button variant="ghost" size="sm" disabled={busy} onClick={handleNewProject} title={t("new_project_tip")}>
            <Plus size={13} /> {t("new_project")}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setShowGallery(true)} title={t("gallery_tip")}>
            <Images size={13} /> {t("gallery")}
          </Button>
          {hasRawStrips && (
            <Button variant="ghost" size="sm" onClick={handleExportRawStrips} title={t("export_strips_tip")}>
              <Scissors size={13} /> {t("export_strips")}
            </Button>
          )}
          <Button size="sm" disabled={!exportable} onClick={handleExport} title={t("export_tip")}>
            <Package size={13} /> {t("export")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            title={settings ? t("provider_tip", { provider: PROVIDER_LABELS[settings.provider] ?? settings.provider, model: settings.providers?.[settings.provider]?.model ?? "" }) : t("settings_tip")}
            onClick={() => setShowSettings(true)}
          >
            <Settings size={13} />
            {settings ? PROVIDER_LABELS[settings.provider] ?? settings.provider : t("settings")}
            {settings && !hasActiveKey(settings) && <span className="text-destructive font-bold">!</span>}
          </Button>
        </div>
      </header>

      <main className="workspace">
        <section className="panel panel-char">
          <CharacterPanel
            character={character}
            cellSize={cellSize}
            busy={busy}
            onChange={setCharacter}
            onCellSize={setCellSize}
            onError={(m) => (m.includes("canceled") ? toast("info", t("toast_gen_canceled")) : toast("error", m))}
          />
        </section>

        <section className="panel panel-anim">
          <StatesPanel
            states={states}
            directions={directions}
            presets={presets}
            selectedId={selectedId}
            canGenerate={!!character.image && hasActiveKey(settings)}
            hasImage={!!character.image}
            busy={busy}
            perspective={character.perspective}
            onStates={setStates}
            onSelect={setSelectedId}
            onGenerate={handleGenerate}
            onGenerateAll={handleGenerateAll}
            onAddCustomBatch={handleAddCustomBatch}
            onGenerateDirectionSet={handleGenerateDirectionSet}
          />
        </section>

        <section className="panel panel-preview">
          <PreviewPanel
            state={selectedState}
            allStates={states}
            directions={directions}
            cellSize={cellSize}
            padFrac={padFrac}
            onPadFracChange={setPadFrac}
            busy={busy}
            onUpdateState={updateState}
            onSelect={setSelectedId}
            onRegenerate={handleRegenerate}
            onExport={handleExport}
          />
        </section>
      </main>

      {showSettings && settings && (
        <SettingsModal
          settings={settings}
          onClose={() => setShowSettings(false)}
          onSaved={async (msg, keepOpen) => {
            const s = await refreshSettings();
            if (!keepOpen && hasActiveKey(s)) setShowSettings(false);
            toast("success", msg);
          }}
        />
      )}

      {showGallery && (
        <GalleryModal onClose={() => setShowGallery(false)} onError={(m) => toast("error", m)} />
      )}

      <Dialog open={confirmNew} onOpenChange={(o) => !o && setConfirmNew(false)}>
        <DialogContent className="w-[380px]">
          <DialogTitle>{t("confirm_new_title")}</DialogTitle>
          <DialogDescription>
            {t("confirm_new_desc")}
          </DialogDescription>
          <div className="row" style={{ justifyContent: "flex-end", gap: 8, marginTop: 4 }}>
            <Button variant="ghost" size="sm" onClick={() => setConfirmNew(false)}>
              {t("cancel")}
            </Button>
            <Button variant="destructive" size="sm" onClick={resetProject}>
              {t("confirm_new_ok")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <div className="toasts">
        {toasts.map((t) => (
          <div key={t.id} className={`toast ${t.kind}`}>
            {t.text}
          </div>
        ))}
      </div>
    </div>
  );
}
