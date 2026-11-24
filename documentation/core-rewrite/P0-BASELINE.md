# P0 baseline: what #747 does with a real Python env

Run on 2026-11-24 from `platform/p0-verify` (main at 6edd548) by a worker of the platform plan chain. P1 is compared
with this file. Each line says what was run and what it showed; what was not run is at the end.

## How it was run

- **Windows 10, Go 1.27.4.** A scratch copy of the repo (`git archive HEAD`) under `%TEMP%\p0scratch\root`, with
  `installer_files` a junction to the developer's real install (Python 3.10.11, gradio 5.49.1, torch, 8 extensions
  installed), `installer_scripts/.gpu` = `NVIDIA GPU` (copied), a `config.json` copied from the real one with
  `server_port` 17910 and `inbrowser` false. `react-ui/out` built with the env's npm (`npm ci`, `npm run
  build:static`). Binary built from this branch. GPU: GTX 1070, CUDA 13.0 driver.
- **Linux, WSL distro `tts-linux-test`.** The same copy in `/root/p0root`, `installer_files` a symlink to an earlier
  install (`/root/install-flow/final`: Python 3.10, gradio 5.49.1, torch 2.11.0+cpu, `.gpu` = `CPU`), a Linux binary
  and a Linux `launcher.test` cross-compiled on Windows, `server_port` 17920.
- Launcher runs: `tts-webui launch -root <copy> -port 179xx -no-browser`, then `POST /api/app/start` with the token
  from `GET /` (the call the launcher page makes).

## Results

| Line | Windows | Linux (WSL) |
|---|---|---|
| Start with the real env, all extensions enabled | gradio ready on 17910 and React on 3000; Gradio `/config` lists 112 tabs, none marked `(!)`; `Huggingface Cache Manager` is a live tab | same: 112 tabs, none `(!)`, Gradio ready on 17920, React 3000 |
| Stop (`POST /api/app/stop`) | public port and 3000 closed, no process of the copy left | ports closed, no python/uv left |
| `LAUNCHER_REAL_ROOT` tests (`go test ./internal/launcher -run TestReal`) | `TestRealExtensionProcessAppliesDecorators` PASS (13 s), `TestRealBridgeStartsAndStopsExtension` PASS (13 s), `TestRealNativeExtensionInstallAndStart` with `tts_webui_extension.piper_tts` PASS (15 s, reinstall from a warm cache) | `TestRealExtensionProcessAppliesDecorators` PASS (6 s), `TestRealBridgeStartsAndStopsExtension` PASS (6 s) |
| `TestRealPipInstallThroughBridge` | not run (needs a throwaway venv and a package entry with one tiny requirement) | not run |
| Native extension install with real torch, through the Go catalog | Extensions page of the launcher, Install on `Piper TTS (uv)`: 2.6 GiB torch download, ended with `Successfully set up virtual environment`; the venv has `torch 2.11.0+cu128`, `torch.cuda.is_available()` True on the GTX 1070 | see "Linux native install" below |
| Launcher page and Vite pages rendered in a browser (built-in browser pane) | launcher setup page and Extensions page rendered; pip list (245 packages after Load), log viewer, model location, server settings, GPU info (GTX 1070, 8 GB, CUDA 13.0), extension lists, remixer, ffmpeg metadata, YouTube tutorials rendered with real data | not run (no browser on the WSL side; the same routes answered 200 on 3000 and on the public port) |
| ffprobe with real input | a 1 s FLAC with tags, uploaded through the real ffmpeg-metadata page: ffprobe output with `duration 1.000000` and the tags `title: P0 test tone` | not run |
| Remixer with real input | the same FLAC loaded as input, copied to two slots, Concatenate: output audio element, duration 2 s | not run |
| A real proxy extension behind the router | Piper (own venv, real torch) started from its Gradio tab: `GET /tts_webui_extension.piper_tts/` 200, 58 KB (plain and over TLS) | see "Linux native install" below |
| `--docker` | not run (binds 0.0.0.0; Windows firewall prompt) | `launch -docker -start`: router on `[::]:7770`, `/` and `/config` 200 on 127.0.0.1 and on the WSL LAN address, clean exit on kill |
| TLS | **failed, fixed, rechecked** (below) | same |

## Faults found

1. **TLS was not wired in the router (fixed in this branch).** With `ssl_keyfile` and `ssl_certfile` in `config.json`
   the public port answered plain HTTP and Gradio itself served TLS, so every page failed (`proxy error ... EOF`,
   the router's HTTP proxy talking to a TLS Gradio). The removed Python proxy never terminated TLS either. Now the
   router serves the certificate and Gradio stays plain behind it. Rechecked on Linux and Windows with a self-signed
   certificate: `https` on the public port verified against the certificate, Gradio's own port plain, the app link is
   `https://127.0.0.1:<port>`, and on Windows a real own-venv extension was started and served over TLS. Not
   checked: a browser with a trusted certificate, Docker with TLS, PKCS#8 encrypted keys (refused with a message).
2. **`Huggingface Cache Manager (!)` (fixed).** `huggingface_hub` 1.33 has no `commands` package. The extension now
   carries its own `ANSI` and `tabulate`.

## Observations to carry into P1 to P4 (not fixed; all in paths that change)

- The Extensions page and `extcatalog` call a native extension **installed as soon as `.venvs/<pkg>` exists**, so
  while the install is still downloading (minutes) the card already says installed. P2 (venv state).
- The venv is created from the main env's `python.exe` (`Using CPython 3.10.11 interpreter at
  ...\installer_files\env\python.exe`): a native venv depends on the main env. P2.
- The native install put `tts_webui_extension_openai_tts_api` into the Piper venv as a dependency. P2.
- Piper started with `[WinError 3] ... 'data/models/piper'` in its log: the folder is missing in a copy made by
  `git archive`. Not checked on an install that has `data/models`.
- GPU Info page: `PyTorch Version` is empty (the Go route cannot ask torch). Cosmetic; P3.
- In the scratch copy the launcher's setup page shows `Install WebUI` with a Run button, because the copy lacks the
  install markers (`.pip_packages` and the rest); this is not a result about a real install.
- The Extensions page needs about 10 s to list (it runs `pip list`).
- `ffmpeg` 4.4.2 in the env cannot encode MP3 (`mp3_mf`), so the test file is FLAC.

## Not run

- The Linux browser views, ffprobe and remixer pages, `TestRealPipInstallThroughBridge` on both OSes, `--docker` on
  Windows, macOS anything, `-race` (needs cgo), Docker image build, a trusted-certificate TLS check.
