# `onnx` platform: design (P8)

Written 2026-11-25. Follows [PLATFORM-DESIGN.md](PLATFORM-DESIGN.md) (the seam) and the `uv` platform
([INTEGRATION.md](../../core/internal/platform/uv/INTEGRATION.md)). **Nothing is built yet.** Every statement about
the engine's behaviour that was not run is marked *(not run)*.

## Pick

**Runtime: sherpa-onnx v1.13.8 (k2-fsa), the prebuilt command-line program `sherpa-onnx-offline-tts`.
First engine: KittenTTS nano 0.8, int8 (`kitten-nano-en-v0_8-int8`), English, run on CPU.**

Why this is a platform and not a `uv` entry: the runtime is a native program and onnxruntime comes inside the archive
as a DLL. No Python, torch, venv or uv is involved, so it fits the plan's rule for `onnx` ("an ONNX runtime that is not
Python").

What was read (2026-11-25, GitHub API and the files themselves):

| Fact | Source |
|---|---|
| **Pinned `v1.13.8` (2026-09-10)**, Windows `shared-MT-Release` archive, 24,805,859 bytes, digest `sha256:6dffdc71...58f6e8df`; a download hashed to the same value. **Not v2.0.3 (the latest):** it dropped espeak-ng, and both the Kitten and the Piper model packages fail there ("No lexicon available" / "phoneme_codepoints is empty", run 2026-11-25). Moving to 2.x needs a lexicon and a phonemizer: a later change | release API, own `sha256sum`, own runs |
| The archive's `bin/` holds `sherpa-onnx-offline-tts.exe`, `onnxruntime.dll`, `onnxruntime_providers_shared.dll` (and ASR tools we do not use) | extracted listing |
| Runtime licence Apache-2.0 | `LICENSE` of the repo |
| Model `kitten-nano-en-v0_8-int8.tar.bz2`, 31,220,690 bytes, digest `sha256:6fa5be85...a83af2fd` (own download hashed the same). Holds `model.int8.onnx`, `voices.bin`, `tokens.txt`, `espeak-ng-data/`, `LICENSE`, `README.md` | `.../releases/tags/tts-models`, extracted listing |
| Model licence Apache-2.0 (`KittenML/kitten-tts-nano-0.8-int8`) | Hugging Face API card data |

Licence note to record in the catalog: the model folder ships `espeak-ng-data/` (espeak-ng is GPL-3.0; I did **not**
verify this from the folder, only from general knowledge, so the claim is *unverified*). We download it at install and
never redistribute it.

### Rejected

| Candidate | Why not |
|---|---|
| Piper voices through sherpa-onnx (`vits-piper-en_US-amy-low-int8`, 21 MB) | Smaller, and the same runtime would run it; second engine after Kitten. A `piper_tts` entry exists already; voice licences vary per voice (not checked). |
| Kokoro int8 (`kokoro-int8-en-v0_19`, 103 MB) | Better quality, five times the download; not needed to prove the platform. |
| `kitten-mini-en-v0_1-fp16` (157 MB), `kitten-mini-en-v0_8` (68 MB) | Larger; nano is enough. |
| Python `onnxruntime` engines (kokoro-onnx, piper1) | A `uv` entry by the plan's own rule, so they would not exercise a platform. |
| The `static` Windows archives (100 to 1000 MB) | Needless size; the `shared` one is 32 MB. |
| `shared-MD-*` | Needs the MSVC runtime on the machine; `MT` links it statically. *(Not tested either way.)* |
| onnxruntime directly from Go (cgo) | Core builds with `CGO_ENABLED=0`; a child process keeps D2 (all inference out of process). |

`extensions.json` has no ONNX-based entry today (grep for `onnx` found none). The existing `kitten_tts` entry is a
Python/torch engine; the new one is a second route to the same model family and does not replace it.

## Platform design

| Question | Design |
|---|---|
| **Install** | `platforms/onnx/`, per D28. Download the pinned archive over HTTPS, verify sha256 against a pin in Go (`pin.go`, like `uv`), extract to `onnx/runtime.new/`, check the probe file `bin/sherpa-onnx-offline-tts(.exe)` exists, swap, write `platform.json` last. Pin per OS and architecture; **Windows x64 only in P8**, other OSes report "not available in this version". Disk-space pre-flight as `uv`. |
| **Entry (model)** | `Create` downloads the model archive into `platforms/onnx/models/<entry>/` (pinned size and sha256), extracts to `.new`, swaps, writes `entry.json` last (`installing.json` before) so an interrupted install is `Partial` and runs again. Removing deletes the folder. |
| **Run** | No long-lived process. One generation = one `sherpa-onnx-offline-tts` process: `Command(e, ["offline-tts", ...])` resolves the program and model arguments, no shell. Text goes by argument or file, output to a temp wav. *(Model load time per call: not run.)* |
| **Cancel** | An install: context cancel stops the download and removes the `.new` folder. A generation: cancel kills the child process; the temp wav is deleted. Because there is one process per generation, cancel is exact. |
| **Models** | Inside the platform folder, one subfolder per entry, never shared; no use of the Hugging Face cache. |
| **CPU / GPU** | CPU only in P8 (`--num-threads`). The archive's onnxruntime has CPU provider; CUDA/DirectML runtimes are separate archives and are **not built**; the platform page says "CPU". |
| **API shape** | The Go side serves OpenAI-style `POST /v1/audio/speech` for the entry (D27), so it appears in the generate page built for `openai`, and Go saves the clip into `outputs/` in the layout the History page reads (reusing the `openai` platform's save path). Params: `input`, `voice` (speaker id 0..N, names from the model's `voices.bin` *(not run: how the names are listed)*), `speed`, wav output. No Gradio, no iframe. |
| **Manifest** | `requires: ["onnx"]`; fields `runtime: "sherpa-onnx"`, `model: {url, sha256, size}`; the pin lives in Go, the catalog only names the model. Name `onnx` is already reserved. |

## What "done" means (demo, Windows; other OSes reported as not run)

1. Fresh scratch folder with no Python, no micromamba, no uv: the launcher's Extensions page offers the engine;
   Install shows the plan (platform, then model), downloads, checks both checksums, and finishes.
2. Generate a clip from the Vite generate page; it plays, is non-silent, and is in History with the right metadata.
3. Cancel an install during the download; run it again; it completes.
4. Cancel a generation (long text); the child process is gone and no wav is left.
5. Remove the entry; the platform stays; the folder is gone.
6. A wrong checksum (tampered pin in a test) leaves nothing installed.
7. A grep/test shows no route returns anything but what the browser needs, and that the engine runs with no network after install.

Not in P8: GPU runtime, other OSes, a second model, streaming, voice cloning.

## Run on 2026-11-25 (v1.13.8, Windows, CPU, 2 threads)

Kitten nano 0.8 int8, "Hello from the ONNX platform.": 63,007 samples, wav written, RTF 0.47 (one process, model load included in the wall time). Piper amy-low int8 also generated (35,584 samples). Both from a scratch folder, not through the core yet.

## Decision (answered)

Rob allowed running the checksummed binary from a scratch folder only (2026-11-25).
