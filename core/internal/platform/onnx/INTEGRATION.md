# platform/onnx: the onnx platform (P8)

Design and reasons for the pick: [PLATFORM-DESIGN-ONNX.md](../../../../documentation/core-rewrite/PLATFORM-DESIGN-ONNX.md).
Seam: D23 and [platform/INTEGRATION.md](../INTEGRATION.md). This file says what was built, what differs from the design
and what was run. **Windows x64 and CPU only.**

## What it is

| Piece | Where |
|---|---|
| `platform/onnx` | `Install` (pinned sherpa-onnx **v1.13.8** `win-x64-shared-MT-Release`, sha256 checked, unpacked beside `runtime.new` and swapped, `platform.json` last); `Create` / `Remove` / `EntryStatus` for a pinned model (`models/<dir>`, `entry.json` last; a folder without it is `Partial`); `Command` (`offline-tts`, no shell); `Generate` (one child process per clip, killed on cancel, temporary folder removed always). |
| `handler.go` | Control API `/api/onnx/engines` and `/api/onnx/speech` (loopback + token, like `platform/openai`). The clip is saved by `openai.SaveClip` (new, `platform/openai/clip.go`: the P5 save code, exported) in the layout History reads. |
| `react_ui_backend/core_onnx.go` | Port 3000 routes `GET /api/go-data/onnx/engines`, `POST /api/go-data/onnx/speech` (switch `core-onnx`). |
| `extcatalog` | `Options.ONNX`; the plan installs the `onnx` platform first (`onnxplatform.go`), installed state of an onnx entry comes from the platform, uninstall removes the model and keeps the platform. |
| Catalog | One entry in `extensions.json`: `tts_webui_onnx.kitten_nano` with `"requires": ["onnx"]`. The model pin (URL, sha256, size) lives in Go (`pin.go`), not in the catalog. |
| Vite | Page `/onnx` (`react-ui/src/pages/onnx.tsx`), header link `ONNX`. Install stays on the launcher's Extensions page. Cancel aborts the request. |

## Differences from the design

- **Runtime v1.13.8, not the latest v2.0.3.** In v2.x espeak-ng is gone, and both the Kitten and the Piper packages fail
  there ("No lexicon available" / "phoneme_codepoints is empty"); run on 2026-11-25. Moving to 2.x needs a lexicon or an
  external phonemizer: a later change.
- Voices are numbered 0 to 7 ("Voice 0"). **The count 8 is not verified** (the runtime accepted sid 0 to 99 without
  error) and the numbers are not mapped to speaker names.
- The text goes on the command line: at most 4000 characters, and it may not start with a dash (it would be read as an
  option). Both are refused with a 400, not changed.
- `--kitten-length-scale` is the only speed option passed, so `Generate` is Kitten-only until a second model kind exists.
- The clip's folder name carries `openai_` (`saveGeneration` prefixes the engine name); the metadata says `onnx_kitten-nano`.
- Not built: free-disk and path-length pre-flight (the uv platform has them), uninstalling the platform from the page,
  an Extensions page row for the platform itself (it appears as a requirement, and as ready once installed).
- `platform.Entry` has no model field, so a model is found by the entry's `package_name`.

## Verification (2026-11-25, Windows 10, Go 1.27.4)

| Line | Result |
|---|---|
| Unit tests (fake runtime = the test binary): install, entry, generate, remove; bad checksum (runtime and model) leaves nothing; cancel of a download then install again; cancel of a generation kills the process and leaves no file; request validation; archive path escape refused; handler token, engines, speech saves a clip, cancelled request saves nothing | pass |
| `TestRealInstallAndGenerate` (`ONNX_REAL_ROOT`): real download of runtime and model with checksums, real clip (non-silent), cancel of a long generation, remove | pass (26 s, cold) |
| Real launcher binary in a scratch folder (`%TEMP%\onnx-e2e`, `--start`, Gradio host not installed, no Python anywhere) | `GET /api/ext/list`: the entry says `Requires: onnx: install the onnx platform: ...` |
| Install from the launcher's Extensions page | the Install button of the row was clicked in the browser pane (by script, `button.click()`, not a pointer click); runtime then model downloaded and checked; entry `installed`, requirement `ready` |
| Generate from the Vite page `/onnx` on port 3000 | text set with the page's form input, Generate clicked (pointer); a 4.0 s clip in `outputs/`, `<audio>` loaded with duration 3.95 s, waveform image loaded |
| History | the page `/outputs/outputs` lists the clip with its audio |
| Cancel a generation (long text, request dropped after 2 s through port 3000) | log `generation cancelled`; no `sherpa` process; no new folder in `outputs/`; `platforms/onnx/tmp` empty (done with `curl -m 2`, not the Cancel button) |
| Remove the entry through the launcher API | model folder gone, platform still `ready` |
| Cancel an install during the model download (client dropped after 1.5 s), then install again | listed not installed, log `Cancelled: ... install again to continue`; the second install completed |
| Pre-existing check `TestRealCatalogRequires` | updated to expect one `onnx` entry |

`gofmt`, `go vet ./...`, `go test ./...` pass in `core`; `tsc -b` shows one error that was there before
(`src/pages/extensions/[name].tsx`, missing generated `extensions/link`); `node build-static.js` built.

## Not run

- Linux, WSL, macOS: the pin refuses them ("Windows x64 only in this version"). Not run.
- GPU runtimes (CUDA, DirectML): not built, not run.
- A wrong checksum against the real downloads (only in the unit tests, with fake archives).
- The Cancel button of the `/onnx` page (the same request abort was done with `curl`).
- The Generate button was not tried with a speed value, and voices other than 0 were only run from the command line
  (sid 0, 3, 7, 8, 15, 99 gave clips).
- A real install that is interrupted while the runtime (not the model) downloads, through the page.
- The espeak-ng data licence (GPL?) was not verified from the model folder.
- Shared files edited here (also touched by other sessions): `extensions.json` (one entry), `extcatalog` (`catalog.go`,
  `install.go`, `installed.go`, `requires.go`, `uvplatform.go`), `launcher/extensions.go`, `launcher/serverpy.go`,
  `react-ui/src/routes.tsx`, `react-ui/src/components/Header.tsx`, `platform/openai/clip.go` (new file).
