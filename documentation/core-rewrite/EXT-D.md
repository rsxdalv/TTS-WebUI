# EXT-D: four Gradio extensions retired (cleanup, no behaviour change)

Branch `cloud/ext-cleanup`, cut from `main` (b844e0a). Companion jobs: EXT-C (model downloader to its own venv) and
EXT-A (HF cache manager in Go), each on its own branch from `main`.

## What changed

Four built-in Gradio tabs are gone. Each was checked against the code first (table below); none was kept.

| Retired | Folder | What replaces it | Checked |
|---|---|---|---|
| `extension_conda_storage_optimizer` | removed | nothing, on purpose | Its only action is `conda clean --all` through `subprocess.run(shell=True)`, and it refuses to run without `CONDA_PREFIX`. The uv platform has no conda env and the legacy install uses micromamba, so on the platform this app is moving to it can only say "No conda environment detected". No Vite page, Go route or `api_name` consumer exists (grep: only `extensions.json`, the folder and LEAF-TABS.md). |
| `extension_simple_remixer` | removed | `frontend/src/pages/tools/simple_remixer.tsx` (+ `functions/remix.ts`), route `/tools/simple_remixer`, Tools menu entry | Same 3x3 slots, "copy from input", clear, concatenate (mix each column, join the columns), send-to-input, now in the browser with Web Audio and a 16-bit WAV download. Known difference, already documented in `remix.ts` and LEAF-TABS.md: mono, at the browser AudioContext's rate (the extension resampled to the highest rate of its slots). No other code imported the extension (the page only mentions the old name in a comment). |
| `extension_youtube_tutorials` | removed | `frontend/src/pages/tools/youtube_tutorials.tsx`, route `/tools/youtube_tutorials` | The page lists the same five videos (ids compared one by one) in `youtube-nocookie.com` iframes. Nothing imported the extension. |
| `extension_ffmpeg_metadata` (the Gradio tab) | removed | `POST /api/go-data/ffmpeg-metadata` (`core/react_ui_backend/go_ffmpeg_metadata.go`) and `frontend/src/pages/outputs/ffmpeg-metadata-page.tsx` | The page posts the uploaded file's name to the Go route, which runs ffprobe with the extension's tag rules (ogg: stream 0 comment; flac: format comment; otherwise the whole ffprobe document). The Go side is the superset: tag names matched case-insensitively, and a file without a usable comment gives the whole ffprobe document where the extension raised. Nothing in `react-ui`, `core/ui` or Python called `/api/gradio/load_ffmpeg_metadata`. |

Other edits that follow from the removals:

- `extensions.json`: the four entries are removed, and with `extension_youtube_tutorials` the now-empty `tutorials`
  group. Text edit of the file only (the file does not round-trip through `json.dump`, so the diff is deletions only).
- `core/react_ui_backend/gradio.go`: the `load_ffmpeg_metadata` entry of the api-name table is gone (its Gradio endpoint
  no longer exists and no page calls it).
- `tts_webui/gradio/all_tabs.py` + `extensions_loader/interface_extensions.py`: a class that resolves to no extension is
  not rendered as a tab group any more. Without this the Gradio host would show a "Tutorials" tab with only a "Main" page
  saying "Select a tab to start". `handle_extension_class` takes an optional list so Go's `resolve` is called once per
  class; the existing test that patches `import_module` is unchanged and passes. The class still loads for any external
  extension that declares `extension_class: tutorials`: the group appears as soon as it has something.
- Comments in `remix.ts` and `go_ffmpeg_metadata.go` say the extension is retired; LEAF-TABS.md says so for conda.
- `requirements.txt` is not touched: `ffmpeg-python` is still used by `extension_decorator_save_ffmpeg` and
  `gradio_iconbutton` by `tts_webui/utils`.

Not changed on purpose: the changelogs (history), the Vite pages, the Go ffmpeg-metadata route.

## Legacy-env installs: how an existing install degrades

These four were built-in extensions, so there is nothing to uninstall: the code is a folder in the checkout and the list
is `extensions.json`. Updating the checkout removes both. The loader asks Go for the list (`resolve`), and Go reads the
merged lists, so nothing else needs to know the old names:

- An install with no copy of the entries elsewhere: the four tabs and the empty Tutorials group just disappear. Nothing
  imports the folders, so nothing fails.
- An old `config.json` that lists one of them in `extensions.disabled` or `extensions.autostart`: `Resolve` (`extcatalog/resolve.go`)
  only checks `slices.Contains` for the entries it lists, so a name that is no longer listed is inert.
- An install whose `extensions.external.json` or catalog clone still carries one of the entries (for example the catalog
  repo): the loader tries `importlib.import_module("extensions.builtin.extension_<name>.main")`, gets
  `ModuleNotFoundError` (a leftover `__pycache__` folder is a namespace package without `main`), and
  `generic_error_tab_advanced` shows the usual "<name> (!)" tab. The app starts. The existing test
  `test_a_failing_import_leaves_an_error_tab` covers that code path with an `ImportError`; the user can remove the entry
  from the external file. That catalog repo is not part of this repository, so its list was not changed here.
- The Vite menu is unaffected: its entries are static and the three pages need neither the Gradio host nor the removed
  folders.

## Evidence (Linux x86-64, CPU runner, 2026-11-26)

| Check | Result |
|---|---|
| `gofmt -l .`, `go vet ./...`, `go test ./...` in `core` (Go 1.27.0 toolchain from `go.mod`) | pass |
| `pytest tts_webui/core_bridge/test_core_bridge.py -k "tab or extension or interface"` in a scratch venv with gradio 5.49.1 | 37 passed |
| New test `test_a_class_with_no_extensions_is_not_a_tab_group` | passes |
| `extensions.json` parses; group counts: outputs 3→2, tools 11→9, `tutorials` gone | yes |
| react-ui `npm ci`, `generate-plugin-imports.js`, `tsc --noEmit`, `npm run build:static`; core/ui `npm ci`, `npm run build` | all pass (Node 22.23) |
| `core/internal/webui/assets` | not regenerated: `bundle.hash` depends only on `core/ui/src/core-app` and the lockfile, neither changed, `core/scripts/bundle-hash.sh` reproduces the committed `bundle.hash` (`32d98955...`), which is what the check 'Build core UI and check committed assets' compares |

## Not run

- No real Gradio host start with this branch (needs the full legacy env or the host venv with torch and ~100 extensions;
  the tab loader and the new group skip were exercised in the unit tests above only).
- The "stale entry in `extensions.external.json`" path was not started for real; it is the existing import-error path,
  covered by a unit test with a fake `import_module`.
- No browser run of the Vite pages (unchanged by this branch).
- Windows and macOS are out of scope.
