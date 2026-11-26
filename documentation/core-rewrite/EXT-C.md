# EXT-C: Model Downloader moves to its own uv venv (native entry)

Branch `cloud/ext-model-downloader-native`, cut from `main` (b844e0a). Companion jobs: EXT-D (cleanup) and EXT-A
(HF cache manager in Go), each on its own branch from `main`.

## What changed

The `extensions.json` entry `extensions.builtin.extension_model_downloader` now reads like the P6 conversions
(Kokoro, MMS, Vall-E-X): `"proxy": "native"` and a `validation` value. Two things are different from them, and both
are on purpose:

| Field | Value | Why |
|---|---|---|
| `proxy` | `native` | The marker of an own-venv entry. `requires` is derived as `["uv"]`. |
| `requirements` | `huggingface_hub` (was a `git+https://...tts_webui_extension.model_downloader@main` URL) | The code is **in this checkout** (`extensions/builtin/extension_model_downloader/main.py`, only `gradio`, `os`, `huggingface_hub`, no `tts_webui` import). The engine harness imports `<package_name>.main` with the install folder as the working directory, so the venv only needs the dependencies. The old URL was never installed (a built-in is not installed). `huggingface_hub` is also a gradio dependency; naming it makes the venv's need explicit. |
| `torch` | `cpu` | The downloader does not use torch, but every own-venv install adds one (`uv.Create`). Pinning the CPU row avoids pulling a CUDA build (about 3.5 GB) for a tool that downloads files. There is no "no torch" row; see "Not done". |
| `validation` | `unvalidated` | It passed install, start and a real download on **Linux CPU**; the Windows run is a separate follow-up. Flip to `full` after it. (P6 uses `full` for "generated real audio on both OS".) |
| `package_name` | unchanged | Renaming would orphan the config entries (`disabled`, autostart) of every existing user and the folder name. |

### The finding that needed Go changes

A package name that contains `builtin` is a built-in **tab** everywhere in the core: it is "always installed" and
`extcatalog.installable` leaves it out, so `find`, `Engine` (the only gate before the browser can start a process),
`Engines()`, install and uninstall all answered "no installable extension". Flipping `proxy` alone would have made the
entry look installed with no venv, and the engine page could never have opened it. A native entry cannot be a built-in
tab, so:

- `platform.BuiltinTab(pkg, proxy)` = built-in name **and** not native. Used by `extcatalog.installable`,
  `RequiresGraph` (a native built-in does count as installed, so it blocks removing `uv` like any engine), the launcher
  Extensions list (`builtin: false`, so the page shows Install/Remove) and the `extui` list.
- `InstalledIn` already tested `Own` before `Builtin`, so a native built-in's state is its venv.
- Python loader (`interface_extensions._is_builtin_tab`): a native built-in gets the "Manage" accordion (Install /
  Update (uv venv), Start with WebUI) like the other engines; before, a built-in skipped it, so a Gradio user could not
  have installed it.
- Tests: `TestNativeBuiltinEntryIsInstallable`, `TestBuiltinTabIsNotANativeBuiltin`, a loader test, and the counter in
  `TestRealCatalogRequires` (40 → 41 own-venv entries).

Nothing else changed: the Gradio tab code, `model_download` api name and `data/models/<type>/<name>` layout are as before
(the process's working directory is the install folder, so the relative `data/models` is the same place).

## Evidence (Linux x86-64, CPU runner, no GPU, 2026-11-26)

Scratch folder `/tmp/c-run/root` = `git archive` of this branch (+ `react-ui/out` copied from a build), launcher built from
this branch, `launch --root /tmp/c-run/root --port 17800 --no-browser`. Nothing in the checkout, `installer_files`,
`.venvs`, `platforms`, `outputs`, `config.json`, `.env` or `data` was touched (`git status` clean afterwards).

| Step | Result |
|---|---|
| `GET /api/ext/list` before install | entry: `proxy native`, `validation unvalidated`, `builtin false`, `installed false`, `requires ["uv"]` (state `missing`, "install the uv platform ...") |
| `POST /api/ext/op/install {"package_name": "extensions.builtin.extension_model_downloader", "gpu": "CPU"}` (cold) | platform first (uv 0.13.6, Python 3.10.11, ffmpeg tool with a checksum), then the venv: "Python 3.10.11, torch row cpu", requirements + compatibility pins; stream ended `{"done":true,"ok":true}`. `.venvs/<pkg>/entry.json`: `platform uv, python 3.10.11, torch_row cpu`; `platform.json` `gpu: CPU` |
| `POST /api/app/start`, then `POST /api/go-data/engines/<pkg>/open` on port 3000 | `starting` → `ready`; `ps`: `.venvs/<pkg>/bin/python -m tts_webui.extensions_loader.proxy_harness`, working directory the scratch root, no `TTS_CORE_*` variable in its environment |
| Behind the router | `GET http://127.0.0.1:7770/<pkg>/` 200; `/<pkg>/config` is Gradio 5.49.1 with api name `model_download` |
| Download | `gradio_client` (`/model_download`, type `proof-test`, repo `hf-internal-testing/config-no-model`, no token) → `ok`; files at `data/models/proof-test/hf-internal-testing__config-no-model/{config.json (637 B), .gitattributes (1175 B)}` plus `.cache/huggingface` metadata, i.e. the same layout the Gradio tab writes |
| Idle stop (`TTS_ENGINE_IDLE_MINUTES=1`, warm cache reinstall) | engine `ready` → `exited` after 82 s idle, process gone, router route 404 |
| `POST /api/ext/op/uninstall` with the engine running | "Stopped the running engine", venv removed (30128 files, 1.3 GB as the venv sees them; 37 MB really freed because the rest are hardlinks into the wheel cache), `/<pkg>/` 404; the downloaded files stay in `data/models/` |
| `gofmt -l .`, `go vet ./...`, `go test ./...` in `core` (Go 1.27.0) | pass |
| `pytest tts_webui/core_bridge/test_core_bridge.py -k "tab or extension or interface or installed_and_version"` (scratch venv, gradio 5.49.1) | pass |
| UI assets | `core/ui` is unchanged, so `bundle.hash` is unchanged; nothing to rebuild |

Environment note (not a repo change): this runner exports `UV_NATIVE_TLS=true`, and uv 0.13.6 prints a deprecation
warning for it on stderr. `extractTarXZ` takes the first line of `uv python find` (stderr included) as the Python path
and failed with `exec: "warning: The UV_NATIVE_TLS environment variable ..."`. The scratch launcher was started with
`env -u UV_NATIVE_TLS UV_SYSTEM_CERTS=true`. Found, not fixed here: `pythonExe` should read stdout only.

## Not run / not done

- **Windows** (and macOS): not run; a separate follow-up. `validation` stays `unvalidated` until then.
- The **Gradio tab** of the entry (the host's "Manage" accordion and its iframe), a **legacy-env** run (an existing install
  with the Gradio host: the tab now needs the uv platform and an install of the entry before it shows the page; this is what P6
  did to the other 40 and it is the behaviour change of this PR), and the Vite **Engines** page (`GET /api/go-data/engines/<pkg>` and the open route
  were used; the engine list route and the iframe were not opened in a browser). The Gradio host and a browser were not started.
- `api-name` routing (`engineOwners`): not added. No Vite page calls `model_download`, and `gradio.go` has no route for
  it, so there is nothing to route. A page that wants it adds both.
- A torch-free row: every own-venv install adds torch + torchcodec. The 1.3 GB venv for a downloader is mostly hardlinks
  into a cache other engines share, but it is the cost of "exactly like P6"; a `none` row (or `torch: ""` meaning no torch)
  would be a platform change.
- Download of a gated repo with a token (no token available), a large repo, and a failed download (the extension returns
  the exception text through Gradio as before; unchanged code, not run).
