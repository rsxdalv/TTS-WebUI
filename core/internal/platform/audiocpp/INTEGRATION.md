# platform/audiocpp: the audiocpp platform (P7)

Plan: [PLAN.md](../../../../documentation/core-rewrite/PLAN.md) item P7, decisions D2, D23, D27. The platform is the audio.cpp
server binary, pinned and checksummed, plus its models as catalog entries. Nothing here is Python. This file says what was
built, what Rob decided, what was run and what was not.

Decisions Rob made for this item (2026-11-25): pin audio.cpp **v0.9.6** with a **CPU row and a Vulkan row**; the demo model is a
small one (**Pocket TTS English, q8 GGUF**); **try the stream disconnect first and stop the process as the fallback** for
cancel; **Windows only**, plus a **Vite page of its own**.

## What it is

| Piece | Where |
|---|---|
| Pins | `pin.go`: release `v0.9.6`, the `windows/amd64` CPU and Vulkan zips with the sha256 the release API lists (the CPU zip was also downloaded and hashed to the same value). Linux and macOS are **not pinned**: nobody ran them, so they are refused with a sentence, never offered. `models.go`: Pocket TTS English q8 (GGUF 121 MiB, three voice embeddings of 5.9 MiB), each file with size and sha256, from `audio-cpp/audio.cpp-gguf` at a **fixed revision** (`047ec541...`), not `main`. |
| Install | `audiocpp.go`: download to `audio.zip.part`, rename only when the sha256 matches, unpack beside `bin/` (`bin.new`), check `audiocpp_server(.exe)` is in it, swap, write `platforms/audiocpp/platform.json` **last**. A cancel or a mismatch leaves no `bin/`, no zip and no state. |
| Models | `entry.go`: one folder per model under `platforms/audiocpp/models/<dir>/`, each file checked against its sha256 (a file gets its final name only after the check, so a cancelled install keeps what finished), `model.json` last. |
| Server | `server.go`: one `audiocpp_server` per install folder, started by the first generation, with a generated `server.json` (loopback only, `lazy_load`, one model loaded, every model in streaming mode). It is restarted when the set of installed models changes, stopped before a model or the platform is removed, and stopped with the app. |
| Generate and cancel | `Server.Speak` asks for `POST /v1/audio/speech` with `response_format: pcm, stream_format: sse`, collects the PCM and wraps it in a WAV. **Cancel closes the stream**; then a probe request with `busy_timeout_ms: 1000` checks the server went idle, and if it is still busy after 8 s the process is stopped (the next generation starts it again). |
| Control API | `handler.go`, `/api/audiocpp/` (loopback, token): `GET status`, `POST speech`, `POST cancel`, `POST stop`. Clips are saved by `openai.SaveClip` (a small exported wrapper added to the `openai` package) in the layout History reads. |
| Browser routes | `react_ui_backend/core_audiocpp.go`, switch `core-audiocpp`: `GET /api/go-data/audiocpp/status`, `POST .../speech`, `POST .../cancel`. No route installs, removes or stops anything. |
| Install UI | The launcher's Extensions page (D30): a `Platforms` row for audio.cpp, an `audio.cpp models` group, a **build select** (CPU 34 MB / Vulkan 84 MB) that is sent as `audiocpp_row`. Install order is platform, then model; removing the platform is refused while a model needs it. |
| Vite page | `react-ui/src/pages/audiocpp.tsx`, route `/audiocpp`, top-level menu entry (it stays visible without the Gradio host): model and voice, text, Generate, **Cancel**, the clip, a link to History. |

## Cancel: what the server really does

Measured on 2026-11-25 against `audiocpp_server` v0.9.6 (CPU build, Windows), not read from the docs:

| Case | Result |
|---|---|
| Model in `offline` mode, plain JSON request, client closes after 3 s | **The run is not cancelled.** A short request sent right after got no answer in 60 s; the long run kept going and finished on its own (server log: started 15:51:29, last chunk 15:52:56), and the next short request was answered in 0.7 s. |
| Model in `streaming` mode, SSE request, client closes after 3 s | **The run stops.** A short request right after was answered in 1.25 s. |
| `GET /health` and `GET /v1/models` while a run is busy | answer at once, so they say nothing about busy. `busy_timeout_ms` on a request does: 503 `server_busy` while busy. |

So the platform configures every model in streaming mode and asks for SSE; a model that cannot stream has no cancel except
stopping the process, which is what the fallback does. The fallback was only run against a fake server (below).

The manifest field `cancel` of PLATFORM-DESIGN section 2 was **not added**: nothing reads it. Cancel is code in this platform.

## Verification (2026-11-25)

Windows 10, Go 1.27.4, a scratch copy of `git archive HEAD` in `%TEMP%\p7fresh` with no Python, no micromamba, no git or Node on the
launcher's path needs; `react-ui/out` built from this branch; launcher on port 18930, router on 18940, UI on 3000. `--no-browser --start`.
Driven through the real launcher page and the real Vite page in the browser pane (the pane was hidden, so by DOM events on the
real pages, as in P5). Cold downloads from GitHub and Hugging Face.

| Line | Result |
|---|---|
| `gofmt`, `go vet ./...`, `go test ./...` in `core` | pass (Windows) |
| Install from the launcher page: Install on the Pocket TTS entry, CPU build: the platform first (34 MB, "Checksum matches 9f4d9956..."), then four model files, each "Checksum matches" | pass |
| A wrong checksum is refused with a real download: `TestRealWrongChecksumIsRefused` (real v0.9.6 zip, pin set to zeros): `checksum mismatch ... got 9f4d9956..., want 0000...`, no `bin/`, no zip, no state | pass |
| Generate from `/audiocpp` (cold server start included): 7.7 s clip in about 6 s; WAV 24 kHz mono 16 bit, 368684 bytes; the player URL answers 200 `audio/wav`; `/api/go-data/outputs/outputs` (what History reads) lists it with the metadata | pass |
| Cancel mid-generation from the page: 40 sentences, Cancel pressed at 4 s while the server was busy; answered in 119 ms; the page said "Cancelled. Nothing was saved."; History still had 1 clip; the **server process was the same** (no kill needed); the next generation answered in 2.1 s | pass |
| Platform removal while a model needs it: "cannot remove audiocpp: required by audiocpp_pocket_tts_en" | pass |
| Remove the model, then the platform, from the page: files gone, `platforms/audiocpp` gone, the clips in `outputs/` kept. Run again with the **server running**: it was stopped first and removal worked | pass |
| Vulkan row chosen in the page's select: 84 MB, "Checksum matches 94c6e1df...", `/health` says `"backend":"vulkan"`, a 3.8 s clip generated | pass (this machine; no check that it was faster) |
| Cancel the model install at 30% of the 121 MiB file, then Install again | the cancel left no `.part` and no `model.json`; the page said "Cancelled"; the second install downloaded all four files and completed |
| App stop (`POST /api/app/stop`) with the server running: no `audiocpp_server` left. Launcher process killed hard with the server running: no `audiocpp_server` left (job object) | pass |

### Only in tests, not run for real

- **The stop-the-process fallback of cancel** (`TestCancelStopsTheServerWhenTheRunDoesNotStop`): a fake server that ignores a closed stream. No real
  model in offline mode was run.
- Install checks with a fake archive over TLS (a bad checksum, an archive with no server, a name with `..`, a cancel during the download, a
  repeat that downloads nothing); the catalog flow with a fake model (`extcatalog/audiocpp_test.go`); the browser routes (`core_audiocpp_test.go`).

### Also checked (Linux, 2026-11-26)

A wrong checksum of the zip and of each model file with real downloads, a 404, a cut-off and a truncated download, each with a retry:
[CHECKSUM-CHECKS.md](../../../../documentation/core-rewrite/CHECKSUM-CHECKS.md). A kept model file is now hashed again, not trusted by its size.

### Not run

- **Linux, WSL and macOS** (Rob: Windows only for P7). Their builds are not pinned.
- CUDA builds (not pinned: they need the separate `cudart` zip), ROCm, Windows arm64.
- A second device on the LAN; the page from another machine.
- Other models, other languages, voice cloning, `voice_ref`, the other audio.cpp routes (ASR, music).
- Disk-space and Windows path-length pre-flight (the uv platform has them; this one does not). The scratch folder was short.
- Vulkan on a machine without a Vulkan driver (what the page says is untested).
- The sha256 values come from the release API of the same publisher that serves the zip: they catch a corrupt or swapped download, not a
  compromised release. There is no signature check.

## Differences from the plan and the design

- No `launch`, `health` or `cancel` manifest field and no `extensions.json` schema change except `requires: ["audiocpp"]` and a new group
  `audiocpp` with one entry. A model is known to the platform by `package_name` (`audiocpp.Models`); a model not in that list cannot be added by
  pasting an entry (the install says the model is not known).
- One server for all installed models, not one process per model: audio.cpp loads models lazily and `max_loaded_models` is 1, so a second
  model swaps the first out of memory instead of using more.
- No idle stop (D31): the server runs until the app stops or a model is removed. The cap on engines is not involved: there is one process.
- The Gradio pages and the `openai_tts_api` extension are not touched. The audio.cpp server also has an OpenAI-style `/v1/audio/speech`; the
  `openai` platform can point at it (`http://127.0.0.1:<port>/v1`) but the port is random and not shown; the Vite page here is the way in.

## Shared files touched (for the merge with P6 and P8)

`core/internal/serverpy/extcatalog/{catalog,requires,install}.go` (one `Options` field, one block in `RequiresGraph`, four small branches in
`install.go`; the rest is in the new `audiocpp.go`), `core/internal/launcher/{extensions,serverpy}.go` (construct the platform, mount the
control API, stop the server on close, list row), `core/internal/platform/requires/requires.go` (the reserved text), `extensions.json` (one
group), `core/internal/platform/openai/{save,clip}.go` (a `Label` and an exported `SaveClip`), `core/ui/src/core-app/ExtensionsView.tsx`
**and the embedded bundle under `core/internal/webui/assets/`** (rebuild it after a merge: `core/scripts/build-webui.*`), `react-ui/src/routes.tsx`
and `components/Header.tsx` (one route and one menu entry).
