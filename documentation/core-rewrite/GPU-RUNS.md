# GPU runs: what the platforms do on a GTX 1070

Run on 2026-11-26 from main at `b844e0a` (P0 to P8), on the one machine that has a GPU. Nothing here changes behaviour;
the GPU-choice question (LAB-46: the page preselects the proposed GPU, the choice is fixed at platform install) stays
**documented and deferred**. Card: LAB-53.

## Short answer

| Question | Result |
|---|---|
| NVIDIA row (`cu128`, what the launcher proposes) on a GTX 1070 | **Does not work.** `torch.cuda.is_available()` says `True`, but the build has no `sm_61` kernels and no cuDNN for it: a matmul fails with "no kernel image", Kokoro on GPU fails. The user sees a generic Gradio error; the real text is only in the engine log. |
| `cu126` row | **Works.** Arch list has `sm_61`, matmul and a cuDNN LSTM pass, Kokoro, MMS and Vall-E-X generate real audio on the GPU. But **no entry in the shipped `extensions.json` asks for `cu126`** (no entry has a `torch` field), so a GTX 1070 user following the page gets `cu128` and a broken GPU engine. |
| Engine cap (default 4) | Works: the fifth engine stopped the least recently used one. |
| Idle stop (off by default) | Works: with `TTS_ENGINE_IDLE_MINUTES=1` an idle engine stopped after 1 min and VRAM fell from about 1030 MiB to 305 MiB. Off by default: confirmed. |
| audio.cpp on Vulkan | Works on the GTX 1070. About **5.3 times faster** than the CPU build (1.35 s against 7.2 s for a 10.6 s clip). Cancel and remove pass. |
| onnx (sherpa-onnx) | **CPU only.** The pinned runtime was built without GPU: `--provider=cuda` prints "Available providers: CPUExecutionProvider. Fallback to cpu!" and exits 0. No GPU archive without CUDA 12 and cuDNN 9 exists for Windows. |

## How it was run

- Windows 10, GTX 1070 8 GB (Pascal, `sm_61`), driver 582.66, `nvidia-smi` says CUDA 13.0. Go 1.27.4.
- Scratch folder `%TEMP%\gpuruns`: `git archive HEAD` of this branch, a launcher built from it, `react-ui` built with
  `npm ci` and `npm run build:static`. No checkout's `installer_files`, `.venvs`, `platforms`, `outputs`, `config.json`,
  `.env` or `data` was touched. The folder was deleted afterwards.
- Ports: launcher 7801, router 7810, UI 7811 (all in 7800-7899). Launcher started with `launch -root <scratch> -port 7801
  -no-browser -start`, stopped by port and PID, never by image name.
- Driven through the launcher's HTTP API (the calls the Extensions page and the Vite pages make): `POST /api/ext/op/install`,
  `POST /api/go-data/engines/<pkg>/open`, `POST /api/go-data/audiocpp/speech`. Engines were called with `gradio_client`
  from the engine's own venv. **No browser pane was used; nothing was clicked.**
- Two changes in the **scratch copy only** (not in this branch):
  1. `extensions.json`: `"torch": "cu126"` added to Kokoro, MMS, Vall-E-X, Piper and Kitten (the `cu126` row is opt-in, see below).
  2. The launcher read `REACT_UI_PORT` nowhere (see "Found"), so a one-line patch made it read `SCRATCH_REACT_PORT`, to keep the UI port out of 3000.
- VRAM numbers are the whole-card `nvidia-smi --query-gpu=memory.used`; under WDDM `--query-compute-apps` shows `[N/A]` per process.
  Idle desktop baseline was 300 to 330 MiB.

## 1. The NVIDIA torch row on this GPU

### What the launcher proposes

```
POST /api/ext/op/install {"package_name":"tts_webui_extension.kokoro"}        (fresh folder, no gpu field)
-> {"error":"choose the GPU for the uv platform before it is installed (proposed: NVIDIA GPU)",
    "needs":"gpu_choice","proposed":"NVIDIA GPU","reason":"NVIDIA GeForce GTX 1070",
    "choices":[{"label":"NVIDIA GPU","row":"cu128","approx_mb":3500},{"label":"Apple M Series Chip","row":"pypi",...},
               {"label":"Intel GPU (XPU)","row":"xpu",...},{"label":"CPU","row":"cpu","approx_mb":400},
               {"label":"Custom (Manual torch install)","row":"pypi",...}]}
```

The proposal is NVIDIA / `cu128`. `cu126` is not among the choices (it is an entry-level opt-in, `torchrows.go`).
The install with `"gpu":"NVIDIA GPU"` gave `platform.json` `gpu: "NVIDIA GPU"` and `entry.json` `torch_row: "cu128"`;
log line: `Python 3.10.11, torch row cu128 (about 3500 MB, shared with every other venv on this row)`.

### `cu128` (the proposed row): Kokoro venv

```
torch 2.11.0+cu128  cuda build 12.8
is_available True
arch_list ['sm_75', 'sm_80', 'sm_86', 'sm_90', 'sm_100', 'sm_120']
device NVIDIA GeForce GTX 1070 (6, 1)
UserWarning: NVIDIA GeForce GTX 1070 with CUDA capability sm_61 is not compatible with the current PyTorch installation.
  The current PyTorch install supports CUDA capabilities sm_75 sm_80 sm_86 sm_90 sm_100 sm_120.
x=torch.randn(64,64,device='cuda'); x@x   ->  AcceleratorError: CUDA error: no kernel image is available for execution on the device
```

So the arch list has no `sm_61`, and `is_available()` is `True` anyway. An engine that guards with
`use_gpu and torch.cuda.is_available()` (Kokoro does) therefore goes to the GPU and fails.

Kokoro through the launcher, `use_gpu=True` (its default), after the engine was open:

```
FAILED in 62.5s   AppError: The upstream Gradio app has raised an exception but has not enabled verbose error reporting.
                  (second try, model cached: FAILED in 2.4s, same text)
use_gpu=False in the same engine:  OK in 7.3s, clip saved and listed
```

The exact error was reproduced in the same venv with the `kokoro` library (the engine's own module needs the Go core
bridge, so it cannot run outside the launcher):

```
KModel(repo_id="hexgrad/Kokoro-82M").to("cuda")  ->
RuntimeError: cuDNN version 91900 is not compatible with devices with SM < 7.5.
Please install a version of PyTorch with a compatible cuDNN version.   (torch/nn/modules/rnn.py flatten_parameters)
```

**What the user sees:** the generic "has not enabled verbose error reporting" error. The cuDNN / `sm_61` text is only in
the engine's log ring buffer (`/api/engines/<pkg>/logs` on the control API; the browser routes on port 3000 deliberately do
not expose logs). Not seen in a browser: the Gradio page's toast was not looked at (no browser run); the `gradio_client`
error above is what the API returns.

### `cu126`

Venv installed after the entry carried `"torch": "cu126"` (`entry.json` `torch_row: "cu126"`, log
`torch row cu126 (about 3000 MB ...)`). A second row is a second multi-GB download: the first Kokoro venv was 5.0 GB as the
venv sees it, and removing it freed 0 KB (hardlinks into `cache/`).

```
torch 2.11.0+cu126  cuda build 12.6  cudnn 91002
is_available True
arch_list ['sm_50', 'sm_60', 'sm_61', 'sm_70', 'sm_75', 'sm_80', 'sm_86', 'sm_90']
device NVIDIA GeForce GTX 1070 (6, 1)
matmul 512x512 on cuda  -> ok (14311.0)
nn.LSTM(...).cuda() forward (cuDNN)  -> ok
KModel(...).to("cuda") + one chunk  -> ok, RESULT OK cuda 10.7s total
```

Result: works. Generation through the engines is in the next section.

### Gap this exposes (not changed)

`extensions.json` on main has **no `torch` field on any entry**, so the only way to reach `cu126` is a catalog edit. The
P6 notes say "Chatterbox (GPU via the opt-in cu126 row)": that was a scratch edit, not shipped. Consequences for a Pascal
(`sm_61`) or Maxwell card on the default flow: the page proposes NVIDIA, installs `cu128`, and every GPU-using engine fails with
the generic error above. This is the LAB-46 area (the page should suggest, not preselect, and a card that `cu128` does not
carry should say so). Options for later, none built: (a) detect compute capability below 7.5 in the GPU panel and
propose `cu126` or CPU; (b) a catalog `torch` field set on the engines that are known to work; (c) make an engine's own
`torch.cuda.is_available()` check also test the arch list. `torch.cuda.get_arch_list()` is the cheap check.

## 2. Kokoro, MMS and Vall-E-X on the GPU (own venvs, `cu126`)

All three were installed through `POST /api/ext/op/install` into `.venvs/<package>` and opened through
`POST /api/go-data/engines/<pkg>/open`, then called with `gradio_client` on `http://127.0.0.1:7810/<pkg>/`.

| Engine | Call | Result | VRAM (whole card) |
|---|---|---|---|
| Kokoro | `/kokoro`, `use_gpu=True` | cold OK in 15.1 s (model load), then 2.2 s warm; clip 4.58 s | 301 -> 1040 MiB |
| MMS | `/mms`, language `eng` (no `use_gpu` switch; the code uses CUDA when `is_available()`) | OK in 13.6 s cold; clip 2.11 s | 1073 -> 1355 MiB |
| Vall-E-X | `/vall_e_x_generate`, `short` mode (CUDA when available) | OK in 211 s **cold, includes model downloads**; clip 2.51 s | 1355 -> 3210 MiB |

Real audio and History: all six clips (4 Kokoro, 1 MMS, 1 Vall-E-X) are in `outputs/` with `.wav`, `.ogg`, `.flac`, `.json`
and `GET /api/go-data/outputs/outputs` (what History reads) listed all of them with their metadata. Peak amplitudes
0.28 to 0.69, durations 2.1 to 4.6 s (read with `scipy.io.wavfile`): none silent. Not listened to.

A speed comparison CPU against GPU for these engines was **not** made (the single CPU Kokoro number, 7.3 s on the first
call, includes a model load and is not comparable). No claim is made.

### Two engines at once, and the cap

- **Two at once:** MMS and Vall-E-X generated at the same moment (two `gradio_client` calls started together): MMS OK in 4.7 s,
  Vall-E-X OK in 6.3 s, VRAM stayed near 2470 MiB. Both ran in the same 6 s window; this shows two engines answering at once on
  one GPU, not a measurement of how much they slow each other down.
- **Cap (default 4, `TTS_ENGINE_MAX_RUNNING` unset):** Kokoro, MMS and Vall-E-X running; Piper opened (4 running, fine);
  Kitten opened as the fifth:

```
extprocs: cap 4 reached, stopping tts_webui_extension.kokoro (used longest ago) to start tts_webui_extension.kitten_tts
extprocs: tts_webui_extension.kokoro exited code=1 state=exited
extprocs: started tts_webui_extension.kitten_tts pid=30404 port=65294
```

  Kokoro was the least recently used and not busy. VRAM 3211 -> 2451 MiB. (Piper and Kitten were on `cu126` for this; they do
  not use the GPU in these runs.) The "every engine is busy" refusal (`ErrCapReached`) was **not run**; it is covered by unit
  tests only.

### Idle stop

Idle stop is off unless `TTS_ENGINE_IDLE_MINUTES` is positive.

- **Off by default:** the first Kokoro engine ran from 03:14 to 03:23:52 (almost 10 minutes) with no stop until the cap stopped it.
- **On (`TTS_ENGINE_IDLE_MINUTES=1`):** launcher restarted with the variable; Kokoro opened, one GPU clip at 03:25:31, then polled
  every 15 s:

```
03:25:31 after GPU generate: 1042 MiB
03:26:02 "state":"ready" VRAM 1056 MiB
03:26:32 "state":"ready" VRAM 1030 MiB
03:26:49 INFO extprocs: stopping tts_webui_extension.kokoro after 1m0s without use
03:27:03 "state":"exited" VRAM 305 MiB
```

  The engine's own status polls (`GET /api/go-data/engines/<pkg>`) did not count as use. Opening the engine again and generating
  worked (OK in 14.0 s, VRAM back to 1031 MiB). A **busy** engine not being stopped (a request longer than the timeout) was not run.
- **Hard kill:** killing the launcher process (`Stop-Process`) with three engines running left no engine process and VRAM was back to
  319 MiB at once (job object).

## 3. audio.cpp (P7), Vulkan and CPU, GTX 1070

audio.cpp v0.9.6, Pocket TTS English q8, voice `alba`. Same text for every run (two sentences, about 10.6 s of speech),
through `POST /api/go-data/audiocpp/speech`, wall time of the HTTP call.

```
Install CPU row:      Checksum matches 9f4d9956..., then four model files "Checksum matches ..."
CPU, warm (4 runs):   7243 7146 7589 7122 ms   (clips 10.56 to 11.28 s)
Remove model, remove platform (platforms/ has only uv left)
Install Vulkan row:   status row "vulkan"; GET /health -> {"status":"ok","backend":"vulkan","version":"0.9.6","git":"04a37f2e",...}
Vulkan, first call:   3826 ms (includes starting the server and loading the model); VRAM 328 -> 656 MiB
Vulkan, warm (4):     1359 1393 1319 1313 ms
```

| | CPU | Vulkan (GTX 1070) |
|---|---|---|
| Warm run, about 10.6 s of audio | about 7.2 s (RTF 0.68) | about 1.35 s (RTF 0.13) |
| Speed-up | | about 5.3 times |
| VRAM | 0 | about 330 MiB more than idle |

The CPU first-call time was not recorded (the first CPU runs were timed with a missing `bc`; the four above are a second batch),
so there is no CPU cold-start number to set against the 3.8 s Vulkan one.

- **Cancel mid-generation (Vulkan):** 80 sentences sent, Cancel after 4 s while the GPU was at 74 % utilisation: `POST cancel`
  answered in **82 ms** `{"cancelled":true,"server_stopped":false}`; the speech call returned HTTP 409
  `{"cancelled":true,"error":"the generation was cancelled"}` after 4200 ms; **the server process was the same** (pid 5760 before
  and after, no kill needed); no new clip folder; the next generation took 1420 ms. This is the same stream-close cancel P7 measured
  on CPU.
- **Remove (Vulkan, server running):** model removed, then the platform removed; no `audiocpp_server` process; VRAM back to 302 MiB.
- Not run: the stop-the-process fallback of cancel against a real offline-mode model (P7: tests with a fake server only); CUDA and ROCm
  builds (not pinned); a cancel on the CPU build in this session; a second GPU or an iGPU choice (the card is the only Vulkan device here,
  `/health` does not name the device; the VRAM rise says it was the GTX 1070).

## 4. onnx (P8): GPU provider?

The pin is sherpa-onnx **v1.13.8**, `win-x64-shared-MT-Release` (onnxruntime 1.28.2). Installed through the launcher
(runtime and Kitten nano model, both "Checksum matches"), then:

```
sherpa-onnx-version.exe        -> sherpa-onnx version : 1.13.8, onnxruntime version : 1.28.2
sherpa-onnx-offline-tts --help -> --provider : Specify a provider to use: cpu, cuda, coreml (default "cpu")
runtime/bin/                   -> onnxruntime.dll (17.8 MB), onnxruntime_providers_shared.dll (105 KB); no onnxruntime_providers_cuda.dll

--provider=cpu   -> RTF 3.630/5.504 = 0.659  (2 threads)
--provider=cuda  -> session.cc:GetSessionOptionsImpl:355 Please compile with -DSHERPA_ONNX_ENABLE_GPU=ON.
                    Available providers: CPUExecutionProvider, . Fallback to cpu!
                    exit 0, clip written, VRAM unchanged (302 MiB)       (the 2.3 s vs 3.6 s difference is run-to-run noise on CPU)
```

So the chosen runtime offers **no GPU provider**, and a request for one falls back silently with exit code 0 (the platform
never passes `--provider`, so it always runs on CPU). Through the platform, `POST /api/go-data/onnx/speech` produced a clip folder
(`outputs/..._onnx_kitten-nan_...`).

GPU archives exist for the same release, read from the GitHub release API (not downloaded, not run):
`sherpa-onnx-v1.13.8-cuda-12.x-cudnn-9.x-onnxruntime1.28.2-win-x64-cuda.tar.bz2` (595,017,373 bytes) and a `cuda-13.x` one (478,110,488
bytes). There is **no DirectML or Vulkan Windows asset**. A CUDA run would need a different pin (another artifact, 0.5 GB), CUDA 12
runtime libraries and cuDNN 9 (this machine's PATH has CUDA toolkit 10.x/11.x only), and whether onnxruntime 1.28.2's CUDA provider
carries `sm_61` kernels was not checked. Because that is a new download outside the pin, it was not tried. Marked **not run**.

## Found, not fixed

1. **Default flow on a Pascal card ends in a generic error** (section 1). Documented; the fix belongs with LAB-46.
2. **No catalog entry uses `torch: "cu126"`**, although the table has the row.
3. **The launcher ignores `REACT_UI_PORT`.** `appenv.PortPlan` reads it, but `superviseConfig` (`launcher/app_supervisor.go`) never
   passes a React port, so the UI always binds 3000 and a second launcher on the machine collides. The scratch run used a patch.
4. **Engine errors are invisible to the browser.** A failed generation shows Gradio's "has not enabled verbose error reporting";
   logs are only on the control API.
5. `--provider=cuda` on a CPU-only sherpa-onnx build is a warning and a CPU run, not an error (relevant if a GPU option is added later).
6. Running Kokoro outside the launcher installed `en_core_web_sm` into the venv (`pip` at runtime, from the engine's spaCy usage).
   Seen once; not investigated.

## Not run

- A browser: no page was clicked and the Gradio error toast was not looked at.
- Kokoro, MMS or Vall-E-X on the `cu128` row other than Kokoro (MMS and Vall-E-X were installed on `cu126` only).
- A CPU against GPU speed comparison for the torch engines; Vall-E-X warm timing.
- Cap with every engine busy; idle stop of a busy engine; the cap or idle stop with audio.cpp (one process, not under the cap).
- `cu126` through the page's GPU panel (the row is not offered there).
- onnx on a GPU runtime; audio.cpp CUDA build; Linux, WSL, macOS; a second GPU.
- Kokoro/MMS/Vall-E-X clips were checked for amplitude and duration, not listened to.

## Cleanup

Launcher stopped by PID (port 7801); `nvidia-smi` back to 318 MiB with no scratch process left; `%TEMP%\gpuruns` deleted.
