# Checksum checks: every pinned download is refused when it is wrong

Run on 2026-11-26 on an Ubuntu cloud runner (Linux amd64, CPU only, Go 1.27.0, outbound HTTPS through the session's
proxy), from `main` at `b844e0a` plus the fixes of this branch. Closes the last open criterion of LAB-51 ("a wrong
checksum was only unit-tested with fake archives"); the board itself was not touched (no board tool here).

Short version: **all 8 pinned downloads that can be installed on Linux were refused with a real download and a wrong
checksum, left no partial file behind, and installed on the retry with the right checksum. 8 artifacts × 6 broken-server
modes (404, cut off three ways, truncated file, empty file) were refused by the real launcher too. All 19 pins match
the bytes their servers really serve.** The run found three small defects; they are fixed here (see
[Defects](#defects-found-and-fixed)).

Evidence (condensed logs, the scripts and the one-line pin edits) is in
[`checksum-evidence/`](checksum-evidence/). Everything below was done in a scratch folder, never in the repo checkout.

## The pinned downloads and where each is verified

All of them go through one `download(ctx, client, url, wantSHA, dest, progress)` per package (`uv/fetch.go`,
`audiocpp/fetch.go`, `onnx/fetch.go`): bytes to `dest.part`, SHA-256 compared, renamed to `dest` only when it matches,
`.part` removed otherwise. The launcher API is `POST /api/ext/op/install` (`{package_name, gpu, audiocpp_row}`), which
streams NDJSON.

| Artifact | Pinned in | Verified by | Installed on Linux? |
|---|---|---|---|
| uv 0.13.6 (6 archives: win amd64/arm64, linux amd64/arm64, mac amd64/arm64) | `uv/pin.go` | `uv.fetchUV` → `uv/fetch.go` | yes, linux/amd64 |
| ffmpeg 8.1.3 (win64, linux64, linuxarm64) | `uv/toolspin.go` | `uv.fetchTool` → `uv/fetch.go` | yes, linux/amd64 |
| MinGit 2.56.1 (win amd64, arm64) | `uv/toolspin.go` | `uv.fetchTool` → `uv/fetch.go` | no (Linux uses the system's git); run through a remapped pin, see below |
| audio.cpp v0.9.6 server (win x64 cpu, vulkan) | `audiocpp/pin.go` | `audiocpp.Install` → `audiocpp/fetch.go` | no (Windows x64 only); remapped |
| Pocket TTS GGUF (121 MiB) and 3 voice embeddings, Hugging Face at a fixed revision | `audiocpp/models.go` | `audiocpp.Create` → `audiocpp/fetch.go`, file by file | model files are OS-independent; the platform they need is remapped |
| sherpa-onnx v1.13.8 runtime (win x64) | `onnx/pin.go` | `onnx.fetch` → `onnx/fetch.go` | no (Windows x64 only); remapped |
| Kitten nano int8 model (`kitten-nano-en-v0_8-int8.tar.bz2`) | `onnx/pin.go` | `onnx.fetch` → `onnx/fetch.go` | needs the runtime; remapped |

Not a pin of this platform code and not covered: the Python that `uv python install` downloads (uv checks its own
download), pip wheels, and the launcher's own installer-zip download (`launcher/download.go`, used by the archive
update): that one has **no checksum at all**. It is not one of the platform pins, so it was left alone and is listed
under [Findings that were not changed](#findings-that-were-not-changed).

## Method

1. `git archive HEAD` of `main` into `work/src-base`, plus the fixes of this branch (so the evidence is of the code that
   is delivered). Never committed; the repo checkout was not used for builds.
2. **Remap copy** (`checksum-evidence/remap.diff`, 22 lines): Linux gets the Windows-only pins to run on, because the
   audio.cpp and onnx platforms refuse any OS but Windows x64 and Linux has no git row. `audiocpp`/`onnx` default to
   `goos=windows` (so they unpack `.exe` files and never run them), and a `linux/amd64` git row serves the MinGit zip.
   No sha is touched. This copy is the "right pins" control of every retry.
3. **Wrong-pin copies**: one per artifact, built from the remap copy with the first hex digit of one sha256 changed.
   `checksum-evidence/variants.txt` shows that each differs from the control by that one line, and only that.
4. Each of the 8 wrong-pin launchers is started on an empty scratch folder (only the repo's `extensions.json` copied in,
   as in a real install) with `--root <scratch> --port N --no-browser`. The token is read from the page, the install is
   a `POST /api/ext/op/install` exactly as the Extensions page sends it, the NDJSON is recorded, the folder is listed
   and `/api/ext/list` is asked what the entry looks like. Then the control launcher is started **on the same folder**
   and the same install is posted again.
5. **Mock copy** (the control with every pinned URL rewritten to a local server, `checksum-evidence/tools/mock`):
   the mock fetches the real bytes once and serves them as `ok`, `404`, `cut` (whole length announced, 40% sent, clean
   close), `cut-chunked` (no length, 40% sent, close), `reset` (40% sent, TCP reset), `short` (a truncated file with an
   honest length) or `empty`. For each artifact the six failures are posted one after the other on one folder, then the
   server is set right and the install is posted again.
6. All 19 pins were hashed against what their servers serve (opt-in tests, below).

`uv`, `ffmpeg` and `git` are installed by the first step of any native engine's install, so those cases post
`tts_webui_extension.kokoro` with `gpu: "CPU"`; after the retry's "The uv platform is installed" line the stream is
closed (the engine's own pip install, which is not under test, is not run). The audio.cpp cases post
`audiocpp_pocket_tts_en` with `audiocpp_row: "cpu"`, the onnx ones `tts_webui_onnx.kitten_nano`.

## Results

### A. Wrong checksum, real download, real launcher (8 of 8 pass)

"Real download" = the NDJSON shows the progress lines of the whole file and the refusal prints the hash of the bytes
that arrived, which is the correct pin (`got` = the real value, `want` = the edited one). Every refusal is
`Failed ... checksum mismatch for <full URL>: got <real sha>, want <edited sha>` and the step plan stops
(`Stopped: step N of M (...) failed`).

| Artifact | Test | Result | Evidence (condensed log in `checksum-evidence/`) |
|---|---|---|---|
| uv 0.13.6 linux/amd64 | `real-uv` | refused after the download (16.8 MB); nothing but an empty `platforms/uv/bin/` remains; no `platform.json`; `/api/ext/list`: platform `missing`. Retry: "Checksum matches f29c3e94…", Python, tools, "The uv platform is installed" | `real-uv.txt` |
| ffmpeg 8.1.3 linux64 | `real-ffmpeg` | uv, Python and (remapped) git installed first; ffmpeg refused after 72 MB; no `tools/ffmpeg`, no `ffmpeg.new`, no `ffmpeg-archive.tar.xz`; no `platform.json`. Retry: uv, Python, git "already there", ffmpeg "Checksum matches 202fe676…", unpacked, platform installed | `real-ffmpeg.txt` |
| MinGit 2.56.1 (zip, served to Linux through the remap) | `real-git` | refused; `tools/` empty; no `platform.json`. Retry: git unpacked, then ffmpeg, platform installed | `real-git.txt` |
| audio.cpp v0.9.6 windows-x64-cpu zip | `real-audiocpp` | refused after the 34 MB download; `platforms/audiocpp/` is empty: no `bin/`, no `audio.zip`, no `.part`, no `platform.json`. Retry: platform and the four model files install | `real-audiocpp.txt` |
| Pocket TTS GGUF (121 MiB) | `real-gguf` | platform installed, GGUF refused after 121 MiB; `models/pocket-en-q8/` is an empty folder, no `model.json`; entry `installed=false`. Retry: GGUF and the three embeddings download, `model.json` written | `real-gguf.txt` |
| Pocket TTS embedding `alba.safetensors` | `real-embedding` | GGUF checked and **kept**, alba refused; the model folder holds the GGUF and an empty `embeddings/`; no `.part`, no `model.json`. Retry: "pocket-tts-english-q8_0.gguf: already there, checksum matches", the three embeddings download | `real-embedding.txt` |
| sherpa-onnx runtime v1.13.8 (win x64 `tar.bz2`) | `real-onnx-runtime` | refused after 24 MB; `platforms/onnx/` is empty (no `runtime/`, no `runtime.download`, no `.new`, no `platform.json`). Retry: runtime and Kitten model install | `real-onnx-runtime.txt` |
| Kitten nano int8 model | `real-onnx-model` | runtime installed, model refused after 30 MB; `models/` is empty, no `entry.json`, status not installed and not partial. Retry: model installs, `entry.json` written | `real-onnx-model.txt` |

Half-done names looked for below the whole install folder after every refusal: `*.part`, `*.download`, `*.new`,
`*-archive*`, `audio.zip`. Found: none, in all 8 cases (the line `half-done names ...: none` is in every log).

### B. Broken server answers, real launcher against a local mock (8 of 8 pass, 48 refusals, 8 retries)

For each artifact the six modes were posted in turn on the same folder; every one was refused with a message that
names the full URL, left nothing behind, and the final install with the server set right succeeded.

| Mode | What the launcher said (example: uv) |
|---|---|
| `404` | `Failed to install the uv platform: download <url>: HTTP 404` |
| `cut` (length announced, then closed) | `... download <url>: the transfer broke after 6719609 of 16799024 bytes: unexpected EOF` |
| `cut-chunked` (no length) | `... the transfer broke after 6719609 bytes: unexpected EOF` |
| `reset` (TCP reset) | `... the transfer broke after 3009932 of 16799024 bytes: read tcp ...: connection reset by peer` |
| `short` (a shorter file, honest length) | `... checksum mismatch for <url>: got 3bff10cb…, want f29c3e94…` |
| `empty` | `... checksum mismatch for <url>: got e3b0c442… (the hash of nothing), want f29c3e94…` |

| Artifact | Test | Result |
|---|---|---|
| uv | `mock-uv` | 6 refused, `platforms/uv/bin/` empty after each, retry installs the platform |
| ffmpeg | `mock-ffmpeg` | 6 refused, no `tools/ffmpeg*`, retry installs |
| git (MinGit) | `mock-git` | same |
| audio.cpp zip | `mock-audiocpp` | 6 refused, `platforms/audiocpp/` empty after each, retry installs the platform and the model |
| GGUF | `mock-gguf` | 6 refused (platform stays installed), model folder empty |
| embedding | `mock-embedding` | 6 refused, only the checked GGUF stays, retry fetches the rest |
| onnx runtime | `mock-onnx-runtime` | 6 refused, `platforms/onnx/` empty, retry installs |
| onnx model | `mock-onnx-model` | 6 refused, `models/` empty, retry installs |

Evidence: `mock-<artifact>.txt` (every request, every folder listing, `/api/ext/list` after each mode).

### C. Every pin against what its server really serves (19 of 19 match)

`TestRealEveryPinMatchesItsDownload` in `uv`, `audiocpp` and `onnx` (opt-in: `UV_REAL_PINS=1`, `AUDIOCPP_REAL_PINS=1`,
`ONNX_REAL_PINS=1`; each file is streamed and hashed, nothing is stored). This covers the pins that cannot be
installed here: uv for Windows, macOS and arm64 (5), ffmpeg win64 and linuxarm64 (2), MinGit amd64 and arm64 (2),
audio.cpp CPU and Vulkan zips (2), the three embeddings and the GGUF (4, with their sizes), the sherpa-onnx runtime and
the Kitten model (2, with their sizes). The ffmpeg linux64 and uv linux64 pins are in the same run. All pass.
These 19 are **hash-checked only**; installing them on their own OS was not run (see Not run).

### D. Unit tests added (fake server, gofmt, go vet and `go test ./...` green in `core`)

| Test | What it proves |
|---|---|
| `TestDownloadRefusesAnythingButThePinnedBytes` (uv, audiocpp, onnx) | `download()` refuses 404, a cut-off, a reset, a chunked cut, a truncated file and an empty file, and a wrong pin; the message names the URL; the folder is empty afterwards. A right pin leaves only the file |
| `TestBrokenUvDownloadsAreRefusedAndTheRetrySucceeds` | `Install` refuses each broken answer, leaves only the empty `bin/`, and installs once the server is right |
| `TestBrokenToolDownloadsAreRefusedAndTheRetrySucceeds` | the same for the tools provider (names the tool and the URL; `tools/` empty) |
| `TestBrokenArchiveDownloadsAreRefusedAndTheRetrySucceeds` (audiocpp) | the same for the server zip; no `bin/`, `audio.zip`, `.part`, `bin.new` |
| `TestBrokenModelFileKeepsTheCheckedFilesAndTheRetryFetchesTheRest` | a broken second file keeps the checked first one, leaves no partial file and no `model.json`, status partial; the retry does not download the checked file again |
| `TestAKeptModelFileOfTheRightSizeButOtherBytesIsDownloadedAgain` | the re-check added to `Create` (below) |
| `TestBrokenDownloadsAreRefusedAndTheRetrySucceeds` (onnx) | runtime and model, every broken answer |
| `TestPythonExeSkipsUvWarningsBeforeThePath` | the `uv python find` fix (below) |

The tests were run against the unfixed code too: the new ones fail there for the reasons below (message without URL,
warning taken as the Python path, damaged file kept).

## Defects found and fixed

1. **ffmpeg could not be unpacked when uv prints a warning** (`uv/tools.go`, `pythonExe`). It took the first line of
   `uv python find` as the Python path, but the platform merges stdout and stderr, and uv warns first whenever
   `UV_NATIVE_TLS` is set (this runner's proxy setup sets it): `unpack ffmpeg-archive.tar.xz: exec: "warning: The
   UV_NATIVE_TLS environment variable is deprecated ...": executable file not found`. The uv platform install then
   failed after the checksum had matched, so the "retry with the right checksum" of the ffmpeg case failed
   (`checksum-evidence/before-fix/`). Now it takes the last output line that is an existing absolute file, and says what
   uv printed when there is none.
2. **A download that broke mid-way did not name the file** (`download` in the three `fetch.go`). The refusal was just
   `read tcp ...: connection reset by peer` or `unexpected EOF`. It is now
   `download <url>: the transfer broke after N of M bytes: <cause>`. A truncated file that ends cleanly was already a
   checksum mismatch.
3. **A kept model file was trusted by its size** (`audiocpp/entry.go`). After a cancelled or refused install, `Create`
   kept every file whose size matched and skipped it. A damaged copy of the right size was therefore accepted and
   `model.json` written over it. Kept files are now hashed (121 MiB in about a second) and downloaded again when they
   do not match.

## Findings that were not changed

- The launcher's own installer-zip download (`launcher/download.go`, `archive.go`; the archive update) is not pinned or
  checksummed. It downloads a release zip and extracts it over the install. Out of scope here; it is the one download of
  the launcher without a check.
- After a refused download the platform folder is left empty (`platforms/uv/bin/`, `platforms/audiocpp/`,
  `platforms/onnx/`, an empty model folder) and the status says `the <name> platform install did not finish: install it
  again`. This is what the platform INTEGRATION files describe ("no `platform.json`: not installed"); no file remains,
  so it was not changed.
- The page sends `gpu` or `audiocpp_row` only for the install that installs that platform; sending them again is
  refused with HTTP 400 (`audiocpp_row is only for an install that installs the audiocpp platform`). Seen while
  scripting the retry; not a defect.
- A later part of a plan keeps the earlier part: with a wrong ffmpeg pin, uv, Python and git stay installed (each was
  checked) and the platform stays "not installed" because `platform.json` is written last.

## Not run

- Windows and macOS hosts: no install of any artifact on its own OS. The Windows-only pins (audio.cpp, sherpa-onnx,
  MinGit) were downloaded, verified, unpacked and refused **on Linux through a remapped pin**; their programs were not
  started. Real Linux does not fetch git; the git cases use the MinGit zip for that reason.
- Installs (not hashes) of: uv for Windows, macOS and arm64, ffmpeg win64 and linuxarm64, MinGit arm64, the audio.cpp
  Vulkan zip. Hash-checked only (C).
- The Extensions page in a browser (the API the page uses was driven directly).
- After the uv platform: the engine's own venv and pip install (the stream was closed after "The uv platform is
  installed"); the real Kokoro engine.
- uv's own checksum of the Python download, pip wheels, the launcher installer zip (see above).
- Cancel during a download through the launcher (covered by existing unit tests and earlier INTEGRATION runs).
- A swapped file together with a swapped pin from the same publisher is outside what these checksums protect (already
  said in `audiocpp/INTEGRATION.md`); it was not tested.

## Reproduce

```bash
# scratch copy of the repo + fixes, then one build per variant (checksum-evidence/tools/mkvariants.py)
CHECKSUM_WORK=/tmp/checksum-work python3 -I documentation/core-rewrite/checksum-evidence/tools/cases.py real
CHECKSUM_WORK=/tmp/checksum-work python3 -I documentation/core-rewrite/checksum-evidence/tools/cases.py mock   # needs tools/mock running on 127.0.0.1:18999
# the 19 pins against their servers
cd core && UV_REAL_PINS=1 AUDIOCPP_REAL_PINS=1 ONNX_REAL_PINS=1 go test ./internal/platform/... -run RealEveryPin
```
