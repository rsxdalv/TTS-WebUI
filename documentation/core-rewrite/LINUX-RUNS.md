# Platform demo lines on native Ubuntu (CPU)

Run on 2026-11-26 from `main` at b844e0a (P0 to P8 merged) by a Claude Code cloud session. Earlier runs of these
lines were on Windows and in WSL ([P0-BASELINE.md](P0-BASELINE.md) and the `INTEGRATION.md` of each platform under
`core/internal/platform/`). This file records what was run on **native Linux**, with the commands, what each line
showed, and what was not run. Small defects found were fixed on this branch (last section); the rest are listed.

## Machine and method

| | |
|---|---|
| Machine | Ubuntu 24.04.5 LTS, glibc 2.39, 4 vCPU, 15 GB RAM, no GPU (`nvidia-smi` absent), x86-64 |
| Toolchain | Go 1.27.4 (the `go 1.27.0` of `core/go.mod`; the runner's own Go was 1.24 and could not build it), Node 22.23.5 |
| Network | An egress proxy with a fast link and a cache in front of it: **install times below are not what a home connection gives**. The runner also has `UV_NATIVE_TLS=true` set (see fix 1) |
| Build | `git archive HEAD` into `/tmp/lr/src` (the checkout was never installed into). `cd frontend && npm ci && node generate-plugin-imports.js && npm run build:static` (CI's `test-react-ui.yaml` steps, 15.7 s with a warm npm cache), `cd core && CGO_ENABLED=0 go build -trimpath ./cmd/tts-webui` (12 s). The embedded launcher UI is the committed bundle |
| Each line | A fresh folder `/tmp/lr/pN` = the source copy + `frontend/out` + the launcher binary, no `.git`, no `installer_files`. Launcher started with `env -i PATH=<limited> HOME=/tmp/lr/homeN TMPDIR=...` plus the runner's proxy and CA variables, `tts-webui launch -root /tmp/lr/pN -port 177NN -no-browser`, then `POST /api/app/start` with the `X-TTS-Token` from `GET /` (the call the launcher page makes) |
| Clicks | Chromium with Playwright on the real pages, served by the real binary. `Save`, `Test`, `Edit`, `Generate`, `Process`, `Cancel` and the GPU `select` were pointer clicks and `selectOption`. **`Install` and `Uninstall` of the Extensions page rows were `button.click()` called from the page's DOM by script, not pointer clicks.** No browser had been run on Linux before (P0 to P8 had none on the WSL side) |
| Limited PATH | `cleanbin` = an empty folder (no python, git, node, npm, uv, micromamba, ffmpeg, not even `sh`). `gitbin` = `git` plus the coreutils `git submodule` needs (`sed basename uname cat grep sh ...`), still no python, node, uv or ffmpeg |

## Results

`pass` = ran and showed what the line says. `fail` = ran and did not. `not run` = not run, with the reason.

| # | Line | Result |
|---|---|---|
| 0 | `gofmt -l`, `go vet ./...`, `go test ./...` in `core` on main, on Linux | **pass** (34 packages ok, 17 s, before any change here) |
| 1 | P4: fresh folder, launcher + `frontend/out`, no micromamba, PATH with no Python/git/Node, Go pages answer | **pass** |
| 2a | P2: install uv and a small CPU engine from the launcher, generate a clip | **pass** (Kitten TTS: clip made from the engine page; Kokoro: clip made and saved) |
| 2b | P2: the clip is in History | **pass for Kokoro**; **Kitten does not write to History** (its tab has no save decorator: the clip stays in Gradio's temp folder; upstream extension, not the platform) |
| 2c | P2: second engine on the same torch row, one inode | **pass** (inode 862031, link count 3 with the cache) |
| 2d | P2: cancel an install during the download, then reinstall | **pass** (cancelled while uv was downloading wheels; listed not installed; reinstall completed). The big torch download was already cached, so this is a cancel in the wheel-download phase, not in a long one |
| 2e | P2: remove one engine, the other still generates, the page shows disk freed | **pass** (running Kitten stopped and removed; Kokoro generated a second clip afterwards; page: `Disk freed: 59.5 MB`) |
| 3a | P3: install the gradio-host from the launcher, Gradio tabs come up behind the router | **pass** (107 tabs, none marked `(!)`, `GET :7770/` 200, 1.5 MB) |
| 3b | P3: install an in-process tab | **pass with `CUDA Toolkit`** (the tab turned from `[Available] CUDA Toolkit` to `CUDA Toolkit` after the restart). **`F5-TTS` failed**: pip `ResolutionImpossible` (F5-TTS `main` against the pinned `gradio==5.49.1`); upstream, listed below |
| 3c | P3: remove the host refused while a tab depends on it | **pass** (`cannot remove gradio-host: required by tts_webui_extension.cuda_toolkit`, with the app running and stopped; with no tab and the app running: `stop the app first`) |
| 3d | P3: no host: public port answers the small page, no 502, own-venv engine serves at `/<pkg>/` | **pass** (`:7770/` 200, 653 B page; Kokoro `/tts_webui_extension.kokoro/` 200, 42 KB, its `/config` answers; 0 lines with 502 in the launcher log) |
| 4a | P5: add endpoint, generate, clip in History (local OpenAI-style mock) | **pass** |
| 4b | P5: wrong URL, wrong key give clear errors | **pass** for wrong path, closed port, unknown host, wrong key, plain http with a key to a public host; **https to an http port says only `cannot reach 127.0.0.1:17778: EOF`** (clear enough to find, not to fix; listed) |
| 4c | P5: no route returns the key | **pass** (`p5-no-key-leak.sh`; the key is only in `platforms/openai/keys.json`, mode 600, folder 700, and in no log) |
| 4d | P5 with the real `openai_tts_api` extension on 7778 | **not run** (it needs the Gradio host and an in-process Kokoro; Kokoro is an own-venv engine now and does not carry that extension) |
| 5a | P7 audio.cpp on Linux CPU | **not run: a Linux build exists but does not start on Ubuntu 24.04** (below). The platform itself refuses Linux with a sentence |
| 5b | P8 onnx on Linux CPU | **refused by main** (pin is Windows x64 only; Linux runtime exists upstream). With a **scratch pin that is not committed**: install, generate, cancel, remove, reinstall all **pass** (below) |
| 6a | `LAUNCHER_REAL_ROOT`: `TestRealExtensionProcessAppliesDecorators`, `TestRealBridgeStartsAndStopsExtension` | **pass** (6.2 s, 4.0 s) on an env that is **not micromamba** (below) |
| 6b | `TestRealNativeExtensionInstallAndStart` (Piper) | **fail, then fixed, then pass** (7.7 s warm): the test looked for `Scripts/python.exe` |
| 6c | `TestRealPipInstallThroughBridge` | **pass** (19.9 s); first time run on any OS |
| 6d | Other opt-in tests: `UV_REAL_ROOT` `TestRealInstall` (17 s), `TestRealTools` (1.6 s), `EXTPROCS_REAL_*` `TestRealExtension` (2.3 s), `AUDIOCPP_REAL_ROOT` `TestRealWrongChecksumIsRefused` | **pass** |
| 6e | `ONNX_REAL_ROOT` `TestRealInstallAndGenerate` | **fail on main** for the same reason as 5b: `not available for linux/amd64 (Windows x64 only)` |
| 7 | Launcher stop signals with an engine running | **pass**: `SIGTERM` and `SIGKILL` to the launcher both leave no engine process and close 3000, 7770 and the launcher port |

### 1. P4: fresh folder, no micromamba, PATH with nothing on it

```bash
/tmp/lr/mkroot.sh /tmp/lr/p4          # source copy + frontend/out + tts-webui
cd /tmp/lr/p4 && env -i PATH=/tmp/lr/cleanbin HOME=/tmp/lr/home4 ./tts-webui launch -root /tmp/lr/p4 -port 17700 -no-browser
```

```
GET /api/health          {"cwd":"/tmp/lr/p4","ok":true,"os":"linux"}
GET /api/steps/status    [{"step":"download","done":true},{"step":"preflight","done":false},{"step":"update","done":true,"detail":"This folder has no archive.json, so its version is unknown and it cannot be updated from here: download the installer zip again."}]
GET /api/app/notice      {"legacy":false}
GET /api/ext/list        platform gradio-host, platform_ready false, "the Gradio host is not installed: install it from the Extensions page (it installs the uv platform first)", 70 entries
POST /api/steps/preflight {"checks":[{"id":"SpacesInPath","status":"pass"},{"id":"GCC","status":"warn","message":"gcc not found ..."}],"ok":true}
POST /api/app/start      {"ok":true}
:3000/ /history /engines /openai /onnx /audiocpp   200 text/html 419 B each (the SPA shell); its JS asset 200
:3000/api/go-data/host-status   {"installed":false,"installable":true,"detail":"the Gradio host is not installed: ...","launcherPort":17700}
:3000/api/go-data/outputs/outputs  200 []     /openai/endpoints 200 []     /onnx/engines 200 [...]
:3000/api/go-data/audiocpp/status  200 {"supported":false,"detail":"no pinned audio.cpp v0.9.6 build for linux/amd64 (only Windows x64 is pinned in this version)",...}
:7770/                   200, 653 B: "The Gradio pages are not installed ... Open the TTS WebUI (it works without them)."
POST /api/app/stop       {"ok":true}; 3000 and 7770 closed
```

The launcher's environment was `PATH=/tmp/lr/cleanbin HOME=/tmp/lr/home4` (read back from `/proc/<pid>/environ`). The
`GCC` warning in the preflight is the only warning; the old `could not record the app processes ... installer_files/app-processes.json`
line P2 saw on a fresh Linux folder did not appear in any launcher log of this run.
`/api/go-data/engines` is 404 (the list route is `engine-list`; not a defect).

### 2. P2: uv, two engines, one inode, cancel, remove

Folder `p2`, `PATH=gitbin` (Linux fetches no git; every engine installs from a `git+https` URL).

```bash
curl -X POST :17701/api/ext/op/install -d '{"package_name":"tts_webui_extension.kokoro"}'
# 409 {"error":"choose the GPU for the uv platform before it is installed (proposed: CPU)","needs":"gpu_choice","proposed":"CPU","reason":"no supported GPU found", choices:[...]}
/tmp/lr/inst.sh 17701 tts_webui_extension.kitten_tts CPU        # POST /api/ext/op/install with "gpu":"CPU"
```

- **First attempt failed** at `Failed to install the uv platform: ffmpeg: unpack ffmpeg-archive.tar.xz: exec: "warning: The UV_NATIVE_TLS environment variable is deprecated ...": executable file not found` (fix 1). After the fix: uv 0.13.6, Python 3.10.11, ffmpeg 8.1.3, torch `2.11.0+cpu` (row `cpu`), torchcodec, then the engine.
- Second failure, mine: with `PATH` holding only `git`, `git submodule update` died (`basename: not found`, `sed: not found`). `gitbin` got the coreutils; not a launcher defect (a real PATH has them).
- **Times** (cold folder, cold HOME, fast link): uv platform + Python + ffmpeg + torch + Kitten = **23.3 s** (`p7`, a clean repeat); first attempt up to its failure 22.1 s, the retry 4.7 s; Kokoro on the shared torch 6.9 s; Chatterbox under 6 s (it finished before the 6 s Cancel I had planned).
- **Disk** (`du`, hardlinks counted once): folder 32 MB before; after uv + Kitten + Kokoro **2.3 GB** (`platforms/uv` 356 MB of which ffmpeg 244 MB, Python 76 MB, uv 37 MB; `cache/` 1.8 GB). One venv alone is 1.6 GB as the venv sees it.
- **Engine page**, Playwright: `/engines` lists the engine, click opens `/engine?pkg=...`, iframe `http://127.0.0.1:7770/tts_webui_extension.kitten_tts/`, `Process` clicked with the text typed: a 6 s waveform in the Gradio output. Kitten wrote nothing to `outputs/`.
- **Kokoro** (`Generate` clicked, device default): `outputs/2026-11-26_01-03-21__kokoro__Kokoro_on_a_nat/` with `.wav` (362458 B, IEEE float, mono 24000 Hz), `.flac`, `.ogg`, `.json` (`use_gpu: false`); `GET :3000/api/go-data/outputs/outputs` lists it; the Vite page `/outputs/outputs` shows the row and its `<audio src=/api/webui-generations/outputs/.../*.ogg>`.
- **One inode**:

  ```
  $ ls -li .venvs/*/lib/python3.10/site-packages/torch/lib/libtorch_cpu.so cache/archive-v0/*/torch/lib/libtorch_cpu.so
  862031 -rwxr-xr-x 3 root root 446534944 .venvs/tts_webui_extension.kitten_tts/.../libtorch_cpu.so
  862031 -rwxr-xr-x 3 root root 446534944 .venvs/tts_webui_extension.kokoro/.../libtorch_cpu.so
  862031 -rwxr-xr-x 3 root root 446534944 cache/archive-v0/V1jHIbxMOMGC-Ey1/torch/lib/libtorch_cpu.so
  ```
- **Cancel** (`node extop.js 17701 tts_webui_extension.vibevoice Install download`: Install clicked, Cancel clicked as soon as `Downloading ...` showed, 3.4 s in, uv was fetching `pylibsrtp` and `cryptography`): page `Cancelled. Install again to continue; downloads are kept.`; `/api/ext/list` `installed:false`; `.venvs/<pkg>/installing.json` left; no uv process. Install again: `Successfully set up virtual environment for VibeVoice (Early Access) with dependencies`.
- **Remove** (Kitten, the engine process running): page log `Stopped the running engine` / `Removed .../.venvs/tts_webui_extension.kitten_tts: 38620 files, 1.6 GB as the venv sees them` / `Disk freed: 59.5 MB (free space of the volume before and after)` / `The wheel cache keeps 2.4 GB that other engines share; the venv's files were hardlinks into it`. `df` before and after agree (59368755200 to 59433635840 free bytes). Kokoro then made a second clip, listed in History.

### 3. P3: the Gradio host

Folder `p3`, `PATH=gitbin`. `Install gradio-host` on the Extensions page, GPU panel, `CPU` selected: **43.3 s cold** (uv, Python, ffmpeg, torch 2.11.0+cpu, `requirements.txt`; the launcher's own log; `p6` repeated it in 44 s). Folder 3.6 GB.

- `POST /api/app/start`; `GET :7770/config` answered 200 10 s after the start; `components` then has **107 `goodtab`**, 18 `goodtabs`, 0 labels containing `(!)`; `GET :7770/` 200, 1505923 B. While Gradio is starting the router logs one `ERROR upstream unavailable path=/ ... connection refused` per request I made (listed).
- In-process tab: `tts_webui_extension.cuda_toolkit` (a pip install into the host, 5 s). After restart the tab list has `CUDA Toolkit` where it had `[Available] CUDA Toolkit`. `F5-TTS`: `ERROR: ResolutionImpossible` (see the log `installer_scripts/logs/pip-install-F5-TTS.log`: F5-TTS `main` and `gradio==5.49.1`). Every other in-process entry in `extensions.json` is `Not available yet` or already part of the host's `requirements.txt`; PyRNNoise and AP-BWE, which P3 used, are own-venv engines since P6.
- Remove: `POST /api/ext/op/uninstall {"package_name":"gradio-host"}` -> `400 invalid: cannot remove gradio-host: required by tts_webui_extension.cuda_toolkit` (app running, and again with the app stopped). After the tab was removed: `400 invalid: stop the app first: the Gradio host cannot be removed while it runs`. With the app stopped, from the page: `Removed /tmp/lr/p3/.venvs/gradio-host: 43451 files, 2.9 GB as the venv sees them` / `Disk freed: 66.5 MB` / `The wheel cache keeps 3.1 GB`.
- No host: Kokoro installed (2.8 s, torch cached), app started: `GET :7770/` 200 653 B (the small page), `POST /api/go-data/engines/tts_webui_extension.kokoro/open` then `state: ready`, `GET :7770/tts_webui_extension.kokoro/` 200 42382 B, its `/config` lists the Kokoro buttons (`Generate`, `Unload Model`, ...). `grep -c "502\|Bad Gateway"` of the launcher log: 0.

### 4. P5: the openai platform against a local server

The stand-in is `mock_openai.py` (30 lines of Python stdlib, port 17778): `POST /v1/audio/speech` requires `Authorization: Bearer secret-key-123` and answers a WAV tone (16-bit mono 22050 Hz, length from the text); everything else is 404. Real extension on 7778: not run (above). No real remote and no real key.

- Page `/openai`: Name `local`, URL `http://127.0.0.1:17778/v1`, model `tts-1`, voice `alloy`, key `x`, `Save`, `Test` -> `local: the endpoint refused the key (HTTP 401): check the key of the endpoint: Incorrect API key provided`. `Edit`: the Key field is **empty** (the key never comes back); the right key typed, `Save`, `Test` -> `The endpoint answered a speech request with 1.0 s of audio.` The text typed, `Generate` -> `<audio src=/api/webui-generations/outputs/2026-11-26_01-16-34__openai_local__A_clip_made_thr/....wav>`; History (`/api/go-data/outputs/outputs` and the Vite page `/outputs/outputs`) lists it with `endpoint: local`, `model: tts-1`. The mock logged `request body: {"input": ..., "model": "tts-1", "response_format": "wav", "voice": "alloy"}` and `auth=yes`.
- Errors (the same calls the page makes, `PUT .../endpoints/<name>` then `POST .../<name>/test`):

  | Case | Answer |
  |---|---|
  | right host, wrong path (`/nope`) | `HTTP 404 for http://127.0.0.1:17778/nope/audio/speech: check the URL (the base normally ends in /v1) and the model name: no such route /nope/audio/speech` |
  | closed port | `cannot reach 127.0.0.1:17999: dial tcp 127.0.0.1:17999: connect: connection refused` |
  | unknown host | `the host "does-not-exist.invalid" was not found: check the URL of the endpoint` |
  | https to an http port | `cannot reach 127.0.0.1:17778: EOF` |
  | wrong key (test and generate) | `the endpoint refused the key (HTTP 401): check the key of the endpoint: Incorrect API key provided` (generate: HTTP 502) |
  | key with plain `http://example.com` | refused at save, 400: `a key is sent with every request: use https:// for example.com (plain http is accepted only for this machine and private network addresses)` |
- No route returns the key: `bash core/scripts/smoke/p5-no-key-leak.sh http://127.0.0.1:3000 /tmp/lr/p4` -> `PASS: no answer and no file holds the key`. My own `grep -rl` of `secret-key-123` and `WRONGKEY-999` over the root, the launcher log and `HOME` found only `platforms/openai/keys.json` (`-rw-------`, in a `drwx------` folder).

### 5. P7 audio.cpp and P8 onnx

**audio.cpp v0.9.6, Linux: not run.** The pin has Windows x64 only; main answers `no pinned audio.cpp v0.9.6 build for linux/amd64 (only Windows x64 is pinned in this version)` on `/api/go-data/audiocpp/status`, and `POST /api/ext/op/install` for `audiocpp_pocket_tts_en` is `400 invalid: audiocpp_pocket_tts_en requires audiocpp (the audiocpp platform is not available here (this version pins only Windows x64 builds))`; the Extensions page disables Install for a requirement that cannot be installed (`blocker(e)` in `ExtensionsView.tsx`; not clicked on Linux).

Evidence that a Linux build exists and that it does not run here: the release workflow at tag `v0.9.6` (`.github/workflows/release.yml`, read from a shallow git clone; the release's asset list is not readable from this session) builds `audio-v0.9.6-bin-ubuntu-x64-cpu`, `-ubuntu-x64-cpu-portable` and `-ubuntu-x64-vulkan`, and the release serves
`audio-v0.9.6-bin-ubuntu-x64-cpu.tar.gz` (70024372 B, sha256 `18f6f745c006eaaca3bbe6955ec2de4964bf29a25fab409c46b23616c65a652a`) and `audio-v0.9.6-bin-ubuntu-x64-cpu-portable.tar.gz` (75883690 B, sha256 `7e447eb6b6c5dffbde9c972c11ff6f4b70442d2301ada2e10e1d9c5ec9c8ac8c`) (HEAD 200; `.zip` names 404). Both were downloaded and unpacked into their own folders:

```
$ ldd audiocpp_server | grep "not found"
./audiocpp_server: /lib/x86_64-linux-gnu/libm.so.6: version `GLIBC_2.43' not found (required by ./audiocpp_server)
$ ldd audiocpp-portable/audiocpp_server | grep "not found"      # the "portable" build too, and its libggml-base.so.0
... version `GLIBC_2.43' not found ...
```

This runner has glibc 2.39. Both builds were made on GitHub's newest `ubuntu-latest` and need glibc 2.43, so they start only on a distribution that new (not Ubuntu 24.04 LTS, the usual CI and server base). I did not build audio.cpp from source (the task is the pinned binary). The `vulkan` Linux asset was not downloaded. **No Linux pin should be added until an older-glibc build exists** (or the release is built on `ubuntu-22.04`/`24.04`); the digests above are from my own downloads, not from the release API.

**sherpa-onnx v1.13.8, Linux: runs, but main refuses it.** `sherpa-onnx-v1.13.8-linux-x64-shared.tar.bz2` exists (28156791 B; HEAD 200 as `linux-x64-static` and `-shared-no-tts` are too; `linux-x64-shared-cpu` and `-gpu` are 404). sha256 `c0bdb7907d3a74bba1d55d22bf4d9fa75586cf1530614ebe88a27b9118e015c4` (my own download; the release API listing is not reachable from this session). It holds `bin/sherpa-onnx-offline-tts` and `lib/libonnxruntime.so` (RPATH `$ORIGIN:$ORIGIN/../lib`), needs at most GLIBC_2.16 and no symlinks, so `extractTarBz2` handles it. Run directly: `./bin/sherpa-onnx-offline-tts` prints its usage.

On main: `POST /api/ext/op/install` for `tts_webui_onnx.kitten_nano` streamed `Failed to install the onnx platform: the onnx platform is not available for linux/amd64 in this version (Windows x64 only)`, **after the list had said it was installable** (`would: install the onnx platform: the sherpa-onnx v1.13.8 runtime (CPU, about 24 MB)`): fix 2. `ONNX_REAL_ROOT` `TestRealInstallAndGenerate` fails with that sentence.

With a **scratch pin in `/tmp/lr/src` only (not committed, not in this branch)**: `runtimes["linux/amd64"] = {URL: ".../v1.13.8/sherpa-onnx-v1.13.8-linux-x64-shared.tar.bz2", SHA256: "c0bdb790...e015c4", Size: 28156791}` and no other change. Folder `p8`, `PATH=cleanbin`:

| Step | Result |
|---|---|
| Install from the Extensions page (runtime, then model, both `Checksum matches`) | **pass, 14.1 s**; `platforms/onnx` 127 MB (runtime + Kitten nano int8 29 MB model) |
| Generate from `/onnx`, text typed, `Generate` clicked | **pass**: 5.05 s clip in 3.3 s, `outputs/2026-11-26_01-20-15__onnx_kitten-nan__Hello_from_the_/*.wav`, listed by History |
| Cancel a generation (3866 characters, `Cancel` clicked 1.5 s in) | **pass**: launcher log `onnx tts_webui_onnx.kitten_nano: generation cancelled`, no `sherpa-onnx-offline-tts` process, no new clip (1 folder in `outputs/`), `platforms/onnx/tmp` empty. I did not see the process while it ran (the page took longer to load than my 1 s probe) |
| Cancel the install (Cancel at 0.7 s; the log was already at `Checksum matches`, so this cancel hit extraction rather than a transfer), then install again | **pass**: `Cancelled. Install again to continue; downloads are kept.`, `installed:false`; the second install completed |
| Remove the entry (page) | **pass**: `Kitten TTS nano 0.8 (ONNX) is removed`; `platforms/onnx/runtime` stays (the platform is kept, as designed) |
| Wrong checksum | **not run** against the real archive (unit tests only, as on Windows) |

If the maintainers want it: the change is that map entry (and the sentence in `ArchiveFor`, `requires.go` and the `INTEGRATION.md` "Windows only"). I left it out because "Windows x64 only" was a stated decision (P8 design, P7 "Rob: Windows only"), and because the digest should come from the release API.

### 6. P0-style: the opt-in real-root tests, without micromamba

No micromamba was installed. The env of `LAUNCHER_REAL_ROOT` is a real Python environment, **but the `micromamba` the legacy-env platform runs is a 6-line shell shim of mine**, so this is not a run of micromamba:

```bash
# p6 = a fresh folder where the launcher installed the gradio-host (uv venv, Python 3.10.11, gradio 5.49.1, torch 2.11.0+cpu)
#      and the engine tts_webui_extension.mms
ln -s /tmp/lr/p6/.venvs/gradio-host /tmp/lr/p6/installer_files/env          # the "legacy env"
cat > /tmp/lr/p6/installer_files/mamba/micromamba <<'EOF'
#!/bin/sh
# micromamba -p <env> run <cmd> [args...]: put <env>/bin first on PATH and exec the command. Nothing else.
envdir="$2"; shift 2; shift       # -p <env> run
PATH="$envdir/bin:$PATH" exec "$@"
EOF
echo -n CPU > /tmp/lr/p6/installer_scripts/.gpu
cd core && LAUNCHER_REAL_ROOT=/tmp/lr/p6 LAUNCHER_REAL_PYTHON=/tmp/lr/p6/installer_files/env/bin/python go test ./internal/launcher -run TestReal -v
```

| Test | Result |
|---|---|
| `TestRealExtensionProcessAppliesDecorators` (default package `tts_webui_extension.mms`, which is an own-venv engine now, so it needed its `.venvs` first) | PASS 6.2 s: `Loaded: decorator_save_flac, ... decorator_save_ogg_generator`, `GET /tts_webui_extension.mms/` through the router 200, stop 200 |
| `TestRealBridgeStartsAndStopsExtension` | PASS 4.0 s: `started {'port': ..., 'pid': ...}`, `ready {... 'state': 'ready'}`, `stopped ['exited']` |
| `TestRealNativeExtensionInstallAndStart` with `LAUNCHER_REAL_NATIVE_PKG=tts_webui_extension.piper_tts` | first run: install `Successfully set up virtual environment for Piper TTS (uv)` (35 s, the uv platform was already installed in `p6`), then **FAIL `no venv python after the install: stat .../Scripts/python.exe`** (fix 3). After it: PASS 7.7 s, router `GET /tts_webui_extension.piper_tts/` 200, stop `exited`. The Piper log has `[Errno 2] No such file or directory: 'data/models/piper'` (see below) |
| `TestRealPipInstallThroughBridge` with a `python -m venv --system-site-packages` env, a `extensions.external.json` entry `tts_webui_extension.tinytest` (`six==1.17.0`) | PASS 19.9 s: `Successfully installed ... gradio-5.49.1 ... six-1.17.0` and `Successfully installed Tiny test dependencies` |

Not covered by this: conda itself (`micromamba create`, `init_mamba.sh`, the Setup tab's mamba/gpu/install steps), and a legacy install's real `installer_files/env`.

## Windows-only by accident: what was found

| Where | What | Status |
|---|---|---|
| `launcher/real_serverpy_test.go` | `Scripts/python.exe` hard-coded in `TestRealNativeExtensionInstallAndStart` | **fixed** (fix 3) |
| `platform/uv/tools.go` | the Python path was the first line of `uv python find`, any uv warning broke the Linux ffmpeg unpack | **fixed** (fix 1); not Windows-only, but only the Linux `.tar.xz` path asks for it |
| `platform/onnx`, `platform/audiocpp` pins | Windows x64 only by decision. Linux: onnx has a build that runs here, audio.cpp's needs glibc 2.43 | **listed**, not changed |
| Process stop, signals | `SIGTERM` and `SIGKILL` to the launcher with an engine running: no engine left, ports closed (Windows uses a job object; nothing Linux-specific was needed) | **pass** |
| Paths | none failed: `.venvs`, `platforms/*`, `outputs` names, hardlinks (`os.Link` into `cache/`) and `LD_LIBRARY_PATH` for the Linux ffmpeg all worked. ffmpeg's shared build was used by the venv's python (`TestRealTools` checks `ffmpeg -version` through the command) | pass |
| Linux git | not fetched (no official static build): with no git on PATH every engine install says `Failed: Kokoro installs from a git URL and git is not on this machine ... apt install git`, before a venv is made (checked in `p4`) | as designed |

## Found, not fixed

1. **audio.cpp and onnx have no Linux pin** (above). A Linux audio.cpp needs a build that runs on Ubuntu 24.04.
2. **Kitten TTS (own-venv engine) writes nothing to History**: the extension's tab has no save decorator. Kokoro does. If the demo line is "clip in History", use Kokoro.
3. **`F5-TTS` (in-process tab) cannot be installed**: pip `ResolutionImpossible` against `gradio==5.49.1`. It is one of the three installable in-process entries left (`cuda_toolkit` works), so P3's "install one in-process tab" has almost nothing to run on.
4. **Piper's own-venv log: `[Errno 2] No such file or directory: 'data/models/piper'`** (also in P0 on Windows). The engine still serves; the extension lists a folder the install does not create.
5. **`https://` to a plain-http endpoint says `cannot reach 127.0.0.1:17778: EOF`** with no hint that the scheme is wrong.
6. **The router logs one `ERROR upstream unavailable ... connection refused` per request while Gradio is still starting**: noise that looks like a fault.
7. **`UV_NATIVE_TLS` is deprecated in uv 0.13.6** (`Use UV_SYSTEM_CERTS`): every uv command prints the warning into the install log when the variable is set (it is on this runner). Harmless after fix 1.
8. `TestRealWrongChecksumIsRefused` (audiocpp) downloads the Windows zip on Linux, harmless.
9. Not run at all: the `Docker` image, `--docker`, `--system-python`, `-race`, TLS (`ssl_*`), an NVIDIA/ROCm row, a legacy conda install, the Setup tab's mamba steps, the update of an archive install on Linux, macOS (out of scope), Windows path-length pre-flight, the wrong-checksum case against real downloads, the real `openai_tts_api` extension on 7778, a real remote OpenAI endpoint.

## Fixes on this branch

Each commit passes `gofmt -l .`, `go vet ./...` and `go test ./...` in `core` (Go 1.27.4, Linux).

1. **uv platform: `pythonExe` reads the path by its shape.** `uv python find` writes its warnings on the stream the answer comes on; with `UV_NATIVE_TLS=true` the deprecation line came first and was used as the Python's path, so unpacking the Linux ffmpeg `.tar.xz` failed with `exec: "warning: The UV_NATIVE_TLS environment variable is deprecated ...": executable file not found in $PATH` and no Linux engine could be installed. The first line that is the absolute path of an existing file is used now. Test `TestPythonExeSkipsUvWarnings` fails without the change (checked).
2. **onnx platform: not offered where there is no pinned runtime.** `Platform.Supported()` as audio.cpp has; `RequiresGraph` offers the platform only when it is supported, and the reserved sentence is the audio.cpp one. Before: the list said installable and the install failed. Test `TestONNXPlatformOnAnOSWithNoPinnedRuntimeIsNotOffered` fails without the change (checked).
3. **`TestRealNativeExtensionInstallAndStart` finds the venv's python with `platform.VenvPython`.**

## Scratch files

Everything ran under `/tmp/lr` (not kept): `mkroot.sh`, `start.sh`, `inst.sh`, the Playwright scripts, `mock_openai.py`, `cleanbin`, `gitbin`, the scratch onnx pin. Nothing was installed into the checkout; `git status` of the checkout showed only the three commits' files.
