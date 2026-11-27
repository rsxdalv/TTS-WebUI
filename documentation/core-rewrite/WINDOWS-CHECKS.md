# Windows checks of the platform work (P0 to P8)

Run on 2026-11-26 from `claude/local-windows-checks-13de83` (main at b844e0a), Windows 10 Home 10.0.19045, Go 1.27.4, by one
session on the developer's machine. The earlier INTEGRATION.md files listed Windows paths as "not run"; this file runs them
and says what happened. A check that could not be run says **not run**, never passed.

## How it was run

- Everything under `%TEMP%\wchk` (scratch). Real install state of every checkout (`installer_files`, `.venvs`, `platforms`,
  `outputs`, `config.json`, `.env`, `data`) was not written. The one scratch folder that reads the developer's real Python env
  (`wchk\legacy\root`) reaches it through a junction `installer_files`, removed with `rmdir` before the folder was deleted.
- Scratch roots are made from the real release archive (section 1).
- Launcher: `go build ./cmd/tts-webui` from this branch, started as `tts-webui.exe launch -root <scratch> -port 79xx -no-browser`;
  calls to the control API carry the token read from `GET /` (`<meta name="tts-webui-token">`). Ports 7900 to 7999 only, plus
  **3000**: the Vite UI port is fixed (finding 5), so a run that starts the app took 3000 for a few minutes. Launchers were stopped by
  PID found from the listening port, never by image name.
- This box: `LongPathsEnabled = 1` (HKLM), Docker CLI 24.0.5 but **no Docker daemon**, GTX 1070, 222 GB free.

## Summary

| # | Check | Result |
|---|---|---|
| 1 | P4 update: swap of a running exe, real archive | **passed** (the real release json and zip exist now and were used) |
| 1 | P4 update: interrupted, cut download, 404, unreachable | **passed**, with **two bugs found and fixed** (findings 1, 2) |
| 1 | Old (legacy-env) install on this version: starts, notice shown | **passed** (notice read from the API) |
| 2 | Path length, deep folder, real torch | **ran**; the pre-flight refuses as documented; what really breaks with long paths on is measured. Long paths **off** not run |
| 3 | `LAUNCHER_REAL_ROOT` tests (4 of them) | **passed**; `TestRealNativeExtensionInstallAndStart` had been **stale since P2 (failed), test fixed** |
| 4 | `--docker` on Windows, firewall prompt | **not run: no Docker daemon** |
| 5 | TLS in the browser pane | the certificate warning path was **seen** (the pane refuses a self-signed certificate; no tool can click through); the HTTPS page, Vite UI and engine over TLS in a browser **not run**; the plain-HTTP iframe against the TLS router **fails** (finding 6) |
| 6 | Stop by port leaves no orphan; engine cap; double start | **passed** (audio.cpp not re-run: P7 ran it) |

Fixes in this branch (`gofmt`, `go vet ./...`, `go test ./...` pass in `core`, Windows): `download.go` (archive.json written last; no half
`release.zip` after a failed download or extraction), two unit tests for those, and `real_serverpy_test.go` (the native-engine test
installs through the control API with a GPU choice).

## 1. P4 update

The release the launcher checks (`https://github.com/rsxdalv/TTS-WebUI/releases/download/v0.0.0/`) **now has**
`tts-webui-installer.json` (`{"commit":"b844e0a6...","built":"2026-11-26T00:38:04Z"}`, HTTP 200, 85 bytes) and
`tts-webui-installer.zip` (69,371,396 bytes; the CI run "Update Full Repository Zip" for b844e0a succeeded). So the real URL was used, not
a local server, for the swap run. The zip holds `archive.json`, `tts-webui.exe` (11.7 MB; `--version`: `tts-webui main (commit b844e0a6...,
built 2026-11-26T00:37:03Z)`), the other launchers, `frontend/out`, the sources and `.git`. The cases that need a server that misbehaves used a
small local Python file server (`TTS_ARCHIVE_URL=http://127.0.0.1:7950/tts-webui-installer.zip`).

Scratch install: the release zip extracted, `tts-webui.exe` replaced by a build of this branch (`--version`: `tts-webui dev (commit none,
built unknown)`), `archive.json` set to a made-up old commit.

| Line | Result |
|---|---|
| Status of the old archive install against the real release | `update` step `done:false`, `"An update is available: b844e0a6 (this install is 11111111)."` |
| `POST /api/steps/update` (real URL) | stream ends `Extracted 1339 entries`, `Extraction complete.`, `The new launcher program was saved as tts-webui.exe.new; it replaces the running one the next time the launcher starts.`, `Update complete.`, `exitCode 0`. After it: `tts-webui.exe` 16,822,272 bytes (old), `tts-webui.exe.new` 11,685,376 bytes, `archive.json` = b844e0a6, status `done:true` |
| Restart (Windows swap) | the old launcher was stopped by port first. At start `tts-webui.exe` became 11,685,376 bytes and `--version` says `tts-webui main (commit b844e0a6...)`; `tts-webui.exe.old` is there, `.new` is gone, the launcher answers on 7901. **Two `tts-webui.exe` processes run: the old one as a wrapper (parent) and the new one (child, the listener).** Stopping the child by port made the wrapper exit too; the next start removed `.old` (the wrapper's image cannot be deleted while it runs) |
| Cut download (server sends 40% of the zip, then closes) | stream: `37% (26244421 / 69371396 bytes)`, `ERROR: writing zip: unexpected EOF`, `exitCode 1`; tracked files untouched (a local edit of `README.md` survived), `archive.json` unchanged, the step still says an update is waiting. **Found: a 27.7 MB `release.zip` stayed in the install folder** (fixed, finding 2) |
| Launcher killed during the extraction (poll `archive.json`, kill the listener 776 ms after the update started) | **Bug found (finding 1).** `archive.json` already held the new commit while `README.md` still had the old text (a local marker) and no `tts-webui.exe.new` existed yet. After a restart `/api/steps/status` said `update` `done:true` with **no detail**, i.e. up to date over a half-updated install, and nothing offers to run it again. Cause: `archive.json` is the first entry of the zip |
| No installer json (stand-in server answers 404 for the `.json`) | `Could not check for an update: http://127.0.0.1:7950/tts-webui-installer.json answered HTTP 404` (`done:true`, amber detail, not "up to date") |
| Release host unreachable (connection refused) | `Could not check for an update: Get "http://127.0.0.1:7999/x.json": dial tcp 127.0.0.1:7999: connectex: No connection could be made because the target machine actively refused it.` |
| Folder with no `archive.json` | it has `.git` (the zip carries it), so: `This folder is a git clone, not an archive install: update it with git pull.` The "no archive.json and no .git" text was not reached (unit test only) |
| Update while the app runs (409) | **not run** for real (an archive install here has no Python to start the app); `TestGoInstallAndUpdateRefuseWhileTheAppRuns` only |
| Old install (legacy-env) on this build | scratch root = the same files, `installer_files` a junction to the developer's real micromamba env (read only), the install markers copied. `GET /api/steps/status`: six steps, all done. `GET /api/app/notice`: `legacy:true` with the MIGRATION.md text and link. `POST /api/app/start`: Gradio ready (`/config` 1.08 MB, 107 `goodtab` components, none marked `(!)`; the real install shows 112, the scratch has fewer extensions). **The notice banner was not looked at in the browser pane this time** (P4 did); only the API answer |

The old-install update by git (`isGitUpdated`, `init_app.js` through micromamba) was not run: it would change the folder's `.git`.

## 2. Path length

`LongPathsEnabled` on this box is 1 (`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`), so the OS limit is lifted here; the case with it
**off** was **not run** (the registry was not changed).

Deep scratch root: 172 characters (`...\wchk\deep\aaaa...\bbbb...\cccc...\root`).

| Line | Result |
|---|---|
| Launcher install of Kitten TTS (CPU) in the deep root | step 1 (uv platform) **works at 172**: uv 0.13.6 (checksum matches), Python 3.10.11 into `platforms\uv\python`, MinGit and ffmpeg tools. Step 2 refused by the pre-flight before the engine got a folder: `Failed: the install folder is too deep: ...\root\.venvs\tts_webui_extension.kitten_tts is 210 characters and the engine's files need about 150 more, past Windows' 259. Move the install to a shorter folder (for example C:\tts-webui)`, `Stopped: step 2 of 2 ... failed`, `ok:false` |
| Launcher install of `gradio-host` in the same root | the same refusal (`...\root\.venvs\gradio-host`) |
| Pre-flight bypassed by hand: `uv venv` at the 210-character path, `uv pip install torch==2.11.0+cpu` (real wheel) | installs in 21 s; the deepest file is **348** characters (`...\torch\include\ATen\native\transformers\cuda\mem_eff_attention\iterators\predicated_tile_access_iterator_residual_last.h`), so file copies and hardlinks work past 260 with long paths on |
| `import torch` in that venv | **fails:** `FileNotFoundError: [WinError 206] The filename or extension is too long. Error loading ".../lib/site-packages/torch/lib/shm.dll" or one of its dependencies.` |
| Same torch wheel, venv folder of length L (short base Python) | `import torch` works at L = 120, 150, 170, 190, 195, 200 and **fails at 205 and 210**. With long paths on the limit is a DLL load limit near 200, not the file limit |
| The managed Python in a deep folder (`uv python install` into a folder of length L, then `import _multiprocessing`) | ok at L = 110, 130, 150, 170 (python.exe 217 characters); **`ImportError: DLL load failed while importing _multiprocessing: The filename or extension is too long` at L = 190** (python.exe 237). The deep root above (platform folder 192) is in that range: `ssl`, `sqlite3`, `ctypes` still load there, `_multiprocessing` does not |

What this says: the pre-flight is strict, and right for a box with long paths off (260 per file). With long paths on it refuses venv paths of
roughly 110 to 200 characters that would work, and it does not read `LongPathsEnabled`. Two limits it does not name: the venv's DLL
dependency load (about 200) and the base Python's folder (about 190). A venv path that passes the pre-flight (at most 109) keeps both far
away, so nothing found is reachable by a user as a bug. The platform install itself (uv, Python, tools) is not pre-flighted and succeeded
at 172. **Not run:** long paths off; audio.cpp and ONNX platforms in a deep root (P7 says it has no path check).

## 3. Opt-in real tests (`LAUNCHER_REAL_ROOT`)

Scratch root `wchk\legacy\root` (the release files, `installer_files` a junction to the real env, `.gpu` and the markers copied,
`config.json` with `server_port` 7903). Throwaway pip env: `installer_files\env\python.exe -m venv --system-site-packages
%TEMP%\wchk\legacy\pipenv`. One-package entry in the scratch root's `extensions.external.json`: `wchk_tiny`, requirement `cowsay`.

```
go test ./internal/launcher -run 'TestRealPipInstallThroughBridge|TestRealBridgeStartsAndStopsExtension|TestRealExtensionProcessAppliesDecorators' -count=1 -v
  LAUNCHER_REAL_ROOT=<root>  LAUNCHER_REAL_PYTHON=<root>\installer_files\env\python.exe
  LAUNCHER_REAL_PIP_ENV=<pipenv>  LAUNCHER_REAL_INSTALL_PKG=wchk_tiny
go test ./internal/launcher -run TestRealNativeExtensionInstallAndStart -count=1 -v
  LAUNCHER_REAL_NATIVE_PKG=tts_webui_extension.piper_tts   (LAUNCHER_REAL_GPU optional, default CPU)
```

| Test | Result |
|---|---|
| `TestRealExtensionProcessAppliesDecorators` | PASS 19.04 s |
| `TestRealPipInstallThroughBridge` | PASS 6.79 s: `Collecting cowsay`, `Successfully installed cowsay-6.1`, `Successfully installed WChk Tiny dependencies`; `cowsay` is in the throwaway env, `import cowsay` fails in the real env |
| `TestRealBridgeStartsAndStopsExtension` | PASS 15.72 s |
| `TestRealNativeExtensionInstallAndStart` (Piper) | **FAIL before the fix** (1.5 s): `/api/extcatalog/install: choose the GPU for the uv platform before it is installed (proposed: NVIDIA GPU) (HTTP 409)`. Since P2 an engine needs the uv platform and `bridge.extcatalog.install(package_name)` cannot pass a GPU choice. **Fixed in the test:** it first installs through the control API with `gpu` from `LAUNCHER_REAL_GPU` (default `CPU`), then runs the bridge install, start, router GET and stop as before. After the fix: **PASS 212.64 s** (cold uv platform, Python, tools and a torch 2.11.0+cpu venv in the scratch root) |

## 4. `--docker` on Windows

**Not run: no Docker daemon.** `docker version` shows the 24.0.5 client and `error during connect ... open //./pipe/docker_engine: The system cannot
find the file specified`. Docker Desktop was not started. The Windows firewall prompt for a non-loopback bind was not seen either (it
is the `--docker` bind on `0.0.0.0`; `launch -docker` without Docker was not run so that no firewall prompt appears on the developer's
machine). The CI job "Create and publish a Docker image" succeeded on main (b844e0a); that is a Linux build, not this check.

## 5. TLS

Self-signed certificate made with `openssl` (RSA 2048, SAN `localhost` and `127.0.0.1`, 7 days), `ssl_keyfile` and `ssl_certfile` in the scratch
root's `config.json`, app started from the launcher (router on 7903, Vite UI on 3000).

| Line | Result |
|---|---|
| App start with TLS | gradio ready on 7903; `GET /api/app/link` answers `https://127.0.0.1:7903` |
| `curl --cacert cert.pem https://localhost:7903/` | 200; `/config` through the router 200 |
| `curl https://127.0.0.1:7903/` without the certificate | `SSL certificate problem: self signed certificate` |
| `curl http://127.0.0.1:7903/` (plain to the TLS port) | 400 |
| Browser pane, `https://127.0.0.1:7903/` | **the pane refuses it:** `ERR_CERT_AUTHORITY_INVALID` ("Retrying will not help ... The user can choose to continue to the site in the Browser pane; no tool can do that for them"). Nothing was rendered, so no screenshot. The warning path exists; the page behind it was **not** seen in a browser |
| Vite UI `http://localhost:3000/` and `/engines` in the pane while the router is on TLS | the pages load; the engine list shows `Piper TTS (uv)` |
| Engine page `/engine?pkg=tts_webui_extension.piper_tts` | the engine starts (`state: ready`, `public_port: 7903`) but the page builds the iframe **`http://localhost:7903/tts_webui_extension.piper_tts/`** (plain HTTP). The router logs `WARN http: TLS handshake error ... client sent an HTTP request to an HTTPS server` and the frame stays empty (screenshot: empty frame under the menu). The engine page over `https://` was **not** looked at in a browser because of the certificate |
| Trusted certificate | **not run** (optional; no trust store was changed) |

## 6. Process stop, engine cap, double start

Process tree of the legacy-env app: `tts-webui.exe` > `micromamba.exe` > `cmd.exe` > `python.exe server_core.py --no-react`. Engines
(`.venvs\<pkg>`) hang under the launcher as `python.exe` (the venv launcher) > `python.exe` (the managed interpreter).

| Line | Result |
|---|---|
| `POST /api/app/stop` | ports 3000 and the app port closed; no `python`, `micromamba` or `cmd` of the scratch folder left |
| Launcher killed hard (`taskkill /PID <listener>`) with the app up | ports closed; `micromamba`, `cmd`, `python` gone (job object; `orphans_windows.go` records nothing by design) |
| Same with an engine running as well (Kitten under the launcher, next to the Gradio tree) | the launcher's whole tree gone (Gradio python and the engine's two python processes), ports closed |
| Engine cap (`TTS_ENGINE_MAX_RUNNING=1`, Piper and Kitten installed, CPU row) | open Piper: `ready`; open Kitten: log `extprocs: cap 1 reached, stopping tts_webui_extension.piper_tts (used longest ago) to start tts_webui_extension.kitten_tts`, Piper `exited` (code 1, Windows stop), Kitten `ready`. A start while every engine is busy was not run for real (`ErrCapReached`: unit tests only) |
| Second `tts-webui.exe` on the same root and the same port | `tts-webui: listening on 127.0.0.1:7902: listen tcp ... bind: Only one usage of each socket address ...`, exit code 1 |
| Second launcher on the same root, another port, app started in both | the first app keeps running; the second app's start fails with `starting the React UI server: listening on port 3000: ... bind: Only one usage ...`, its status is `stopped`, no stray process from it |
| audio.cpp server | **not re-run.** P7's INTEGRATION.md has app stop and a hard kill that left no `audiocpp_server`; the job object is the same mechanism |

## Findings

1. **Fixed: an interrupted update looked complete.** `archive.json` was written first, so a killed launcher left the new commit over old files and
   the update step said nothing. `extractZip` now writes it last (`TestExtractZip_ArchiveMarkerIsWrittenLast`). Not re-run against a real
   interruption after the fix (the scratch folders had been deleted); the unit test covers it.
2. **Fixed: a failed download or extraction left `release.zip`** (27.7 MB here) in the install folder (`TestDownloadAndExtract_CutDownloadLeavesNoZip`).
3. **Fixed (test only): `TestRealNativeExtensionInstallAndStart` could not pass since P2** (409, GPU choice). The Python bridge still cannot pass `gpu`,
   so a Gradio tab's Install button for an engine cannot install the uv platform on a folder that lacks it; the tab shows the 409 text. That follows
   P2's design (the Extensions page asks for the GPU).
4. **Path length:** the pre-flight is conservative and ignores `LongPathsEnabled`. With long paths on, real torch loads up to a venv path of about 200 and the
   managed Python up to a folder of about 190; the pre-flight stops at 109. Raising it when the registry key is 1 is Rob's choice; nothing was changed.
5. **The Vite UI port is 3000 and cannot be moved here:** `REACT_UI_PORT=7904` in `.env` and in the process environment did not move it (the launcher
   bound 3000). Two launchers cannot both start the app, and a check like this one occupies 3000.
6. **TLS: the engine iframe uses `http://` while the router is on TLS,** so the engine page is empty. It needs the scheme of the public port (the
   `open` answer has `public_port` but no scheme). Not fixed: it changes the Vite UI and the go-data answer, and Rob decided the iframe stays on the router
   port (LAB-45).
7. After an update the swapped launcher runs as a child of the old one (two processes). Stopping the child ends both; what happens when only the parent is
   killed was not tried.

## Not run

Docker on Windows (no daemon) and the firewall prompt, long paths **off**, a trusted certificate in a browser, the HTTPS page and engine over TLS in a
browser, update with the app running (real), `--system-python`, macOS, Linux, `-race` (needs cgo), audio.cpp and ONNX in a deep folder, the git-based
update of an old install, the real interruption run after the fix, the legacy notice banner in a browser.

## Scratch

`%TEMP%\wchk` (`deep`, `legacy`, `lenprobe`, `pyprobe`, `upd`) was deleted after the runs, the `installer_files` junction first with `rmdir`; the real
`installer_files\env\python.exe` was checked afterwards. Ports used: 7901 to 7905, 7910, 7911, 7950, 7999 (refused on purpose), and 3000.
