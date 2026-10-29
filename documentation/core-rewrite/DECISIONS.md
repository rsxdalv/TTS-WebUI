# Decisions

Settled decisions for the core rewrite. Add new ones at the bottom with a date; do not rewrite old ones,
supersede them.

| Id | Date | Decision | Why |
|---|---|---|---|
| D1 | 2026-10-28 | The core is written in **Go**, pure Go, `CGO_ENABLED=0`. | The core is I/O and process supervision; nothing needs to be linked in. Go gives trivial cross-compiles, `context` cancellation, good `os/exec` and reverse-proxy support, and the maintainer can review it. Rust's advantages (in-process ML, Tauri) are not used by this design. A Tauri shell could still run the Go core as a sidecar later. |
| D2 | 2026-10-28 | **All inference is out-of-process.** Engines are Python venvs, native binaries (audio.cpp, whisper.cpp, llama-server) or remote APIs. | Crash isolation (CUDA OOM, ggml segfaults), no GPU build matrix in the core, engines update independently. Cost: about 300–500 MB VRAM CUDA context per engine process, handled by idle stop and a resident cap. |
| D3 | 2026-10-28 | One venv per engine, **no shared torch at runtime**; disk is shared through **uv hardlinks** from a cache on the same volume. | Keeps projects from breaking each other while saving most of the disk. Shared torch in one process would share the crash domain and dependencies for little gain. |
| D4 | 2026-10-28 | **Side-by-side rewrite.** Legacy stays runnable; each area is first re-driven by Go in legacy-compat mode, then replaced natively behind a flag, then the default flips at a milestone. | Lets old and new be tested against each other at every checkpoint. |
| D5 | 2026-10-28 | Engines are described by an **engine manifest** extending `extensions.json` entries (`kind`, install, `launch`, `health`, `api`, `ui`, `cancel`, `keep_alive`). | One supervisor and router for every engine kind; audio.cpp is just another entry. |
| D6 | 2026-10-28 | The UI becomes a **Vite SPA embedded in the binary** (`go:embed`). Node exists only in development and CI. | Removes the Next.js server and Node from user installs; UI and core ship together, so they cannot skew. |
| D7 | 2026-10-29 | **tts-webui-ignition is vendored** into `launcher/` with `git subtree` (history kept), then its logic is moved into Go behind a platform adapter; its UI is kept. | Ignition is the modern launcher UX; its logic is thin shell calls that map cleanly to Go step endpoints. |
| D8 | 2026-10-29 | Go module path `github.com/rsxdalv/TTS-WebUI/core` (matches the remote's capitalisation). Go 1.27. | Set in C0. |
| D9 | 2026-10-29 | The core's HTTP server binds to 127.0.0.1, requires a per-run token on `/api/*`, and checks `Host` and `Origin`. No generic "run a shell string" endpoint, ever. | The server starts processes; any web page could otherwise reach it via localhost or DNS rebinding. |
| D-open-1 | 2026-10-29 | **Core public port: 7700** (fixed default, overridable by `--port`). | No collision with any legacy port (3000, 7767, 7769, 7770, 7771, 7778, 27771+). provisional (director) |
| D-open-2 | 2026-10-29 | **Engine manifest schema**: accept the C5.1 draft schema when the checkpoint produces one; reviewed before merge into `core/rewrite`. | Most reversible; defer schema lock until the draft exists. provisional (director) |
| D-open-3 | 2026-10-29 | **App distribution after git pull**: pinned `tts-webui` package installed by uv into the main env. | Decouples app releases from git history, enables reproducible installs. provisional (director) |
| D-open-4 | 2026-10-29 | **Launcher UI placement**: merge into the `ui/` Vite SPA as the `/setup` route during C6.2. | Single embed, one dev server, shared design system. provisional (director) |
| D-open-5 | 2026-10-29 | **Update signing**: minisign. Throwaway test key checked into `core/internal/selfupdate/testdata/` for CI; real key custody stays with the human. Never generate or store a release signing key in CI or the repo. | Lightweight, reproducible, fits the Go stdlib. provisional (director) |
| D-open-6 | 2026-10-29 | **Python version for new installs**: keep 3.10.11 until P4 is done, then per-engine. | Safe conservative default; avoids breaking existing extension venvs. provisional (director) |
| D10 | 2026-10-29 | **Supersedes D-open-4 for now.** M1 ships the `launcher/` Vite build embedded in the binary, using a `core` platform adapter. Merging the launcher into `ui/` as `/setup` waits until C6 is real. | Fastest route to a usable Go launcher; `ui/` is still a scaffold. |
| D11 | 2026-10-29 | **Verification rules** (PLAN.md): `done` needs a recorded end-to-end smoke against the real binary; `built` is the status for unit-tested but unexercised work; research must cite what it read. | Run 1 marked placeholder and guessed work as done. |
