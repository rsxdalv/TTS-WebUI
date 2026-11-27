# Node graph ("Chains") — spec

Status: **M0–M2 implemented** on this branch (see section 12); M3 and M4 are not started. Replaces the original Next.js prototype in this PR (`nodes-ui/`, Rete + Next + `new Function` nodes), which cannot run on the current architecture (Vite `react-ui` served by the Go core; Next.js removed in D21).

## 1. Problem and scope

Today the only multi-step workflow is `frontend/src/pages/pipeline.tsx`: a fixed "generate with one model, then optionally post-process with one of rvc / demucs / vocos". Users who want more must run tabs by hand and re-upload files:

- text → TTS → RVC → trim/gain → save
- same text through several engines or voices, compared side by side
- one source clip through several effects (stems, RVC voices) at once

A **chain editor** is a page where these steps are boxes joined by cables, run with one button. It is a *front-end orchestration* of operations the app already has. It is not a new backend, not a DAW, not a general programming tool.

Non-goals (v1): real-time audio, timeline/sequencing, user-written code nodes, sharing graphs that carry secrets, server-side scheduled runs, a new Go service or port.

## 2. Core vs separate extension

Rule used: **lightweight and generic → main app; heavy, engine-specific or dependency-bringing → external extension.**

| Piece | Where | Why |
|---|---|---|
| Graph data model, validation, JSON (de)serialisation | main (`frontend/src/chains/`) | Tiny, pure TS, no deps. |
| Executor (topological run, caching, cancel, progress) | main | Same; it is the part that must be correct and tested. |
| Node registry (`registerNode`) | main | The seam extensions plug into. |
| Canvas page (route `/chains`, lazy-loaded) | main | A `lazy()` route chunk costs nothing on other pages (see `routes.tsx`). One new dependency (§4). |
| Built-in nodes wrapping existing `functions/*` (Kokoro, Bark, RVC, Demucs, Vocos, OpenAI speech, history/file input, preview) | main | They only call code already in `react-ui`; no new weight. |
| Browser audio nodes: gain, trim, concat, mix, fade (Web Audio `OfflineAudioContext`) | main | Zero dependencies. |
| Nodes for a specific third-party engine installed as an extension | **that extension** | Registered at load; absent when the extension is absent. |
| Anything needing a new package (WASM DSP, spectrogram editor, MIDI) | **external extension** | Keeps the main bundle and `package-lock.json` flat. |
| Graph templates / example library | catalog / extension | Content, not code. |

Re-evaluate after the spike (§9): if the canvas library's lazy chunk is more than about 150 KB gzipped, move the *canvas page* (not the model/executor) to an external `@tts-webui/chains` package and keep the model, executor and registry in main.

Extension seam today: `@tts-webui/*` packages listed in `frontend/src/extensions/package.json` are linked by `generate-plugin-imports.js` into `link.ts` and rendered at `/extensions/<name>` with only a `Button` prop. A node-contributing extension needs one addition: the main app exposes `window`-free registration, i.e. the extension package default-exports `{ page?, nodes?: NodeDef[] }` and `link.ts` consumers call `registerNode` for each. This is a small, separate change and is tracked as milestone M3.

## 3. Architecture

```
frontend/src/chains/
  types.ts        Graph, NodeInstance, Edge, Port, ValueType
  registry.ts     registerNode / getNode / listNodes
  validate.ts     schema + cycle + type checks, unknown-node refusal
  execute.ts      run(graph, {signal, onProgress}) -> results
  cache.ts        per-node result cache keyed by hash(type, params, inputs)
  nodes/          built-in node definitions
  ui/             Canvas, NodeShell, Cable, Inspector, Palette
frontend/src/pages/chains.tsx   route + Template
```

### 3.1 Data model (JSON, data only)

```ts
type ValueType = "audio" | "text" | "number" | "file" | "params";
type Graph = {
  version: 1;
  nodes: { id: string; type: string; x: number; y: number; params: Record<string, unknown>; bypass?: boolean }[];
  edges: { id: string; from: [nodeId: string, port: string]; to: [nodeId: string, port: string] }[];
};
```

- Audio values are the existing `GradioFile` shape (`{name, url, …}`) so previews, history and metadata keep working.
- A graph file contains **no code and no secrets**. API keys stay in the Go core's OpenAI endpoint store (`/api/go-data/openai/endpoints`, "the key is never in it"); an OpenAI node references an endpoint *name*.

### 3.2 Node definition

```ts
type NodeDef = {
  type: string;                       // "tts.kokoro", "fx.gain"
  label: string; category: string;
  inputs: Record<string, ValueType>;
  outputs: Record<string, ValueType>;
  params: ParamSpec[];                // drives the generated controls (slider, select, text, seed…)
  run(ctx: RunCtx, inputs, params): Promise<Record<string, unknown>>;
};
```

`ParamSpec` generates the node's controls from the existing UI kit (`components/ui/slider`, `select`, `input`, `GenericSlider`, `SeedInput`), so nodes look like the rest of the app. `RunCtx` carries `signal: AbortSignal`, `progress(fraction, text)` and `log`.

### 3.3 Backend access (no new endpoints in v1)

| Need | Route used |
|---|---|
| Engine generation | `POST /api/gradio/<fn>` through the existing `generateWith*` functions (Go proxy to the Gradio host) |
| OpenAI-style speech | `POST /api/go-data/openai/speech` with an endpoint name |
| Input from history/outputs | `/api/go-data/outputs/...` |
| Upload / audio fetch | existing Gradio file proxy (`gradio_file_proxy.go`) |

Generated clips are saved to `outputs/` by the existing decorators, so the History page already lists them. If a node needs a route that does not exist, it is added to the Go `react_ui_backend` and CONTRACTS.md first; the UI never talks to anything else.

### 3.4 Execution

- Topological order; independent branches run in parallel, but engine calls are **serialised per engine** (one GPU model at a time) behind a small queue so a fan-out graph does not run N models at once.
- Result cache per node keyed by hash of (type, params, input value ids). Re-running after changing one knob re-runs only that node and its descendants. A "random seed" param makes the node uncacheable unless fixed.
- Cancel = `AbortController`; in-flight `fetch` is aborted, downstream nodes are marked skipped.
- Errors stop only the dependent subgraph; the failing node shows the message inline.
- Never auto-runs: loading or importing a graph only displays it. Running is an explicit click.

## 4. Canvas library decision

Options compared (measured metadata via `npm view`, 2026-11-26; bundle sizes **not yet measured**):

| | Rete v2 | React Flow (`@xyflow/react`) | Self-made |
|---|---|---|---|
| Licence | MIT | MIT | – |
| React 19 | rete-react-plugin 2.1.2 peers include 19 | peers `react >=18` | n/a |
| Extra deps | `styled-components` (peer, required by render plugin), `usehooks-ts`, 4–5 `rete-*` packages | `zustand`, `classcat`, `@xyflow/system` | none |
| Nodes as our own React components | Yes, via custom render | Yes, the default model: a node *is* a React component | Yes |
| Pan/zoom/select/minimap/handles/a11y basics | Area plugin, mostly manual | Built in (pan, zoom, multi-select, keyboard focus, minimap, snap grid) | Must write all |
| Dataflow engine | Included (`rete` + engine), but ours must be async, cached, GPU-serialised and cancellable, so we write it regardless | None (good: ours is separate) | Ours |
| Controlled/serialisable state | Own graph objects; need adapter to our JSON | State is plain `nodes[]`/`edges[]` arrays, trivially our JSON | Ours |

**Decision: React Flow for the canvas; our own model and executor; no Rete.**

Reasons:
1. The UX goal is "our proper knobs and inputs inside the node". Both libraries allow it, but in React Flow that is the *default* path (a node is any React component), so the Radix-based UI kit drops in unchanged. With Rete we would write a custom render pipeline and add `styled-components` to a codebase that uses Tailwind and emotion.
2. Rete's engine is not the hard part for us; the executor requirements in §3.4 (async, cache, per-engine queue, cancel) mean we write it either way. That removes Rete's main advantage.
3. Our graph JSON is plain data; React Flow's controlled arrays are the same shape, so no adapter.
4. Self-made pan/zoom/selection/keyboard/touch handling is a long tail of bugs for no differentiating value. The *look* is ours either way (custom nodes and edges).
5. It is lazy-loaded, so the cost is limited to `/chains`.

Risk to confirm in the spike: React Flow's real gzipped chunk size, and that its CSS does not clash with Tailwind 3.3.2 preflight. If either fails, fall back to a self-made minimal canvas (SVG cables + absolutely positioned nodes + pointer events, ~800 lines) behind the same `Canvas` interface, so the choice is reversible.

## 5. UX direction: "Cubase, not Blender"

- **Signal flows left to right.** Inputs on the left edge, outputs on the right; cables are smooth horizontal curves, colour-coded by type (audio / text / number). No free-floating sockets on arbitrary sides.
- **Nodes are modules, not widgets lists.** Fixed-width card with header (name, bypass, status LED), the controls always visible (sliders with numeric entry, selects, seed field), and an inline waveform with play button on audio outputs. No collapsing menus inside nodes.
- **Insert like a channel strip.** Drop a node on a cable to splice it in (like inserting an effect); delete a node with Ctrl-click-delete and the cable heals.
- **Bypass and A/B.** Per-node bypass passes audio through; "compare" nodes play two inputs for A/B listening (this is the fan-out use case).
- **Snap grid, aligned lanes, auto-layout** button (left-to-right layered layout), so graphs stay readable.
- **Palette** is a searchable list grouped by category (Source, Generate, Convert, Process, Output); also reachable by double-click on empty canvas and by typing.
- **Keyboard and screen readers:** nodes are focusable, arrow keys move, Enter opens "connect to…" menu so wiring does not need a mouse; status changes announced with `aria-live`; contrast follows the existing Tailwind theme.
- Dark/light follow the app theme. Start from a **template** (e.g. TTS → RVC → Save), never a blank canvas.

## 6. Security

- No `eval`, `new Function`, or remote code. Node behaviour comes only from registered `NodeDef`s. (The prototype's Custom JS node is dropped permanently.)
- Imported graphs are validated (`validate.ts`): known node types only, known ports, type-compatible edges, no cycles, numeric bounds, size cap (e.g. 500 nodes / 2 MB). Unknown types render as inert "missing node" placeholders and block Run.
- Graph JSON never contains keys, absolute paths or URLs to external hosts; OpenAI nodes hold an endpoint *name*.
- All requests go to same-origin `/api/...`; no direct browser calls to third-party hosts (the prototype called `api.openai.com` with a key typed into a node).
- Extensions that register nodes are already trusted code (they are installed packages); the spec adds no extra trust boundary.

## 7. Testing

- Unit (pure TS, Node): validate (cycles, type mismatch, unknown node), executor (order, parallel branches, cache hit/miss, cancel, error isolation) using fake nodes; JSON round-trip.
- Component: node controls render from `ParamSpec`; connect/disconnect via keyboard.
- CI stays as `.github/workflows/test-react-ui.yaml` (`tsc --noEmit`, `build:static`) plus a test step for `chains/`. The repo's `react-ui` has no test runner today, so M1 adds one dev dependency (Vitest, already native to Vite) and nothing at runtime.
- Manual/GPU: one real chain (Kokoro → RVC → save) per release, recorded in the PR.

## 8. Persistence

v1: localStorage (`chains:<name>`), JSON export/import to file. v2 (needs Go route, therefore a CONTRACTS.md change): saved graphs under `outputs/` or the data dir via `/api/go-data/chains`.

## 9. Milestones

- **M0 – spike (½ day):** add `@xyflow/react`, lazy `/chains` route with 3 dummy nodes. Measure `build:static` size delta (gzip of the chains chunk, and confirm main chunk unchanged). Check CSS clash. Go/no-go on §4; otherwise fall back to the self-made canvas.
- **M1 – model + executor:** `types`, `validate`, `execute`, `cache`, `registry`, Vitest tests. No UI.
- **M2 – canvas + built-ins:** page, node shell, cables, palette, inspector; nodes: Text, Number/Seed, Audio file, History clip, Kokoro, Bark, OpenAI speech, RVC, Demucs, gain/trim/concat/mix, Preview/Compare, Save. Two templates.
- **M3 – extension seam:** node registration from `@tts-webui/*` packages; one real extension node as proof; docs for extension authors.
- **M4 – persistence v2 and polish:** Go route for saved graphs, auto-layout, a11y pass.

## 10. Open questions for the maintainer

1. Is a GPU-serialised per-engine queue acceptable, or should the Go core own queuing (is there already a queue in the engine supervisor to reuse)?
2. Should Demucs/RVC be exposed as nodes through the existing Gradio functions, or wait for native engine routes from the Go rewrite?
3. Budget for the main-bundle chunk (the 150 KB gzip trigger in §2 is a placeholder).
4. Keep the route name `/chains` or surface it inside the existing Pipeline page as a replacement?

## 11. What this replaces

The prototype in this PR added 50 files (15,135 lines) under `nodes-ui/`: a separate Next app on port 3002, Rete, a Next API proxy, 8 nodes (two calling a backend) and an eval-based Custom JS node. None of it is reused except ideas: typed sockets (audio/text/number), tabs of graphs, JSON import/export. The prototype was the previous tip of this branch (`670f1e0`); GitHub keeps it reachable by that SHA for a while, but it is not part of the new history.

## 12. Implementation status and findings (as built)

Everything is in `frontend/` (no Go, Python or workflow-runtime change; one CI step added).

**Built (M0–M2):** `frontend/src/chains/` (model, validation, executor, cache, registry, DSP, graph IO, layout, templates, 14 nodes), the canvas under `chains/ui/`, the lazy route `/chains` and a header link. Nodes: Text, Number, Audio file; MMS, Kokoro, OpenAI-style TTS; RVC, Demucs; Gain, Trim, Fade, Join, Mix (Web Audio, no dependency); Preview, Compare A/B, Show text. Three templates.

**Not built yet** (listed in this spec, deliberately left out of this pass):
- Bark node (its parameter set is large; needs its own form), and a History/outputs clip picker. Audio files are entered as a URL/path.
- Drop-a-node-on-a-cable to splice, and the keyboard "connect to…" menu. Nodes are focusable and `Delete` works, but wiring still needs the pointer.
- Persistence beyond localStorage and file export/import (M4 needs a Go route).
- M3, the extension node-registration seam. `registerNode` exists and is the intended seam; nothing wires `@tts-webui/*` packages to it yet.
- Per-engine queues beyond one `gradio` lane and one `openai` lane. The spec's open question about reusing a Go-side queue is unchanged.

**Measured:**
- New route chunk: 98.9 KB gzip JS + 2.7 KB CSS, lazy-loaded. The main `index` chunk is unchanged (79.2 KB gzip before, 79.1 KB after). That is under the 150 KB trigger in section 2, so the canvas stays in main.
- `@xyflow/react` 13.0.3 plus `vitest` are the only new packages (vitest is dev only). Attribution link is left on.

**Tests:** `npm test` runs 59 Vitest tests (executor, validation, cache, DSP, graph IO, layout, API errors/timeouts). In a browser (Playwright, Chromium) the page was driven through a mock backend for: adding nodes, refusing a type mismatch and a cycle, replacing a cable, autosave and reload, templates, error isolation, Stop, bypass, export/import, refusing a graph containing a `custom.js` node, and upload of browser-made audio before RVC. A second run used a real MMS model (`facebook/mms-tts-eng`, CPU) behind the same `/api/gradio/mms` contract: real 6.5 s speech, Gain of -6.02 dB halved the peak, Trim gave 1.00 s, and a downstream change reused the seeded generation without a second request. **Not tested:** the real Go core and Gradio host, RVC/Demucs/Kokoro/OpenAI with real models or endpoints, screen readers, Safari/Firefox, touch.

**Things learned that change earlier assumptions:**
1. `frontend/tailwind.config.js` only scanned `pages`, `components` and `app`. Classes used only under `src/chains/` were silently missing. The chains folder is now in `content`.
2. React Flow 13.0.3 ignores a numeric `padding` in `fitView` (0.15 behaved as 0 px, so graphs touched the canvas edge and edge auto-pan swallowed drags). Percent strings such as `"12%"` work.
3. A server-side node cannot read a browser `blob:` URL. Browser nodes output blob URLs; server nodes upload them through the existing `POST /api/upload` first (`toServerUrl`). Uploaded clips accumulate in `file-input-cache/` like any other upload.
4. The Go gradio route can leave a request unanswered when the Gradio tab does not declare a parameter (see `TestGradioHangsWhereTheOriginalNeverAnswers`). Stop only stops waiting (the Go call keeps running, by design), so every request also has a 15 minute timeout.
5. Generation nodes default to a random seed, which makes them non-cacheable. Fix the seed to reuse a result.
