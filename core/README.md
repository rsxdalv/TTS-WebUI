# TTS WebUI core (Go)

A single exe that sets up and runs TTS WebUI: it serves a local setup page,
runs the install steps (download, pre-flight, micromamba, GPU choice, install),
then starts and stops the app.

**Rule: pure Go, `CGO_ENABLED=0`. Third-party dependencies need a recorded decision** (so far: `golang.org/x/sys`, and the official MCP SDK, D22).

## Run

```sh
tts-webui launch            # serves http://127.0.0.1:7700, uses the exe's folder as the install root
tts-webui launch --root D:\tts --port 7700 --no-browser
tts-webui --version
tts-webui mcp               # MCP server on stdio, see below
```

## MCP server

`tts-webui mcp` lets an MCP client (Claude Desktop, an IDE, an agent) list models and generate speech. It calls the
OpenAI-compatible API, so start that extension first (default `http://127.0.0.1:7778/v1`; change it with
`--backend URL` or `TTS_MCP_BACKEND`).

stdio, for a client that starts the server itself:

```json
{ "mcpServers": { "tts-webui": { "command": "/path/to/tts-webui", "args": ["mcp"] } } }
```

HTTP, opt-in, loopback only, bearer token required (at least 16 characters):

```sh
TTS_MCP_TOKEN=<your secret> tts-webui mcp --http 127.0.0.1:7779
```

The client then connects to `http://127.0.0.1:7779/` with `Authorization: Bearer <your secret>`.

## Build and test (from `core/`)

```sh
go vet ./...
go test ./...
test -z "$(gofmt -l .)"
CGO_ENABLED=0 go build -trimpath -o bin/tts-webui ./cmd/tts-webui
```

## Development

Two loops, run side by side:

```sh
# Go: rebuild and restart the launcher on every Go change (port 7700, install root = the repo).
pwsh -NoProfile -File core/scripts/dev.ps1                    # finds a Go new enough for go.mod, installs air if missing, runs it

# Setup page (core/ui): hot reload against that launcher.
cd core/ui && npm run dev                    # open http://127.0.0.1:5173/
```

The launcher starts the React UI by itself, so `frontend/`'s dev server has an API right away; Gradio waits for its button. `--no-autostart` turns that off, `--start` starts both, `--no-react` leaves the React UI out.

The React UI is served by the router on the public port (`server_port` of `config.json`, 7770): `/` is the UI and its API, the Gradio pages are under `/gradio-legacy/` (Gradio gets `GRADIO_ROOT_PATH`, the router strips the prefix), extensions stay at `/<package>`. The old layout, the React UI on its own port 3000 (`REACT_UI_PORT`) and Gradio at `/`, is still there with `TTS_FRONT_IN_ROUTER=0`.

With `air` (it runs the launcher with `-dev`) the React UI on http://127.0.0.1:7770/ shows what `npm run dev` in `frontend/` serves (Vite on 3002, `TTS_FRONTEND_DEV_URL` to move it), with hot reload, while its API, uploads and the Gradio pages at `/gradio-legacy/` stay the server's. No `frontend/out` is needed. Opening Vite's own http://127.0.0.1:3002/ works too, its `/api` goes to 7770.

`dev.ps1` exists because an old Go earlier on PATH, or a stale `GOROOT=C:\Go` in a session that started before the Go upgrade, fails with "package cmp is not in std" and rewrites `go.mod`/`go.sum`; the script puts a matching Go first, resets `GOROOT` and sets `GOFLAGS=-mod=readonly`. On Linux/macOS run `cd core && air` with a current Go.

`npm run dev` proxies `/api` to `LAUNCHER_URL` (default `http://127.0.0.1:7700`),
fixes the Host and Origin headers and adds the launcher's per-run token, which it reads
from the launcher's index page. After a launcher restart the first request may get a
403; reload the page. The built setup page that `go:embed` serves is still
`scripts/build-webui.sh` (commit its output).

The React UI has its own dev server: `cd frontend && npm run dev` (port 3002, proxies
`/api` to the Go server on the public port, or `VITE_API_PROXY`).

## Layout

- `cmd/tts-webui/` - entry point (`launch`, the default, and `mcp`)
- `internal/mcp/` - the MCP server: two tools over the OpenAI-compatible API ([spec](../documentation/core-rewrite/MCP.md))
- `internal/launcher/` - the setup steps and the app supervisor
- `react_ui_backend/` - the Go server for the React UI (mounted on the router at `/`; port 3000 only with `TTS_FRONT_IN_ROUTER=0`): serves the static export (`frontend/out`, built by Vite with `npm run build:static`) and answers the API routes (`TTS_GO_ROUTES` picks which; `TTS_GO_PROXY=0` is an error now that Next.js is gone)
- `installer_scripts/` - Go ports of `installer_scripts/*`
- `internal/switches/` - the switches that turn the Go ports on
- `internal/server/` - local HTTP server (token, host/origin checks, SSE)
- `internal/procs/` - start, stop and kill a process tree
- `internal/preflight/` - system checks (path with spaces, exFAT, long paths, conda, MSVC)
- `internal/gpu/` - the GPU labels the installer accepts
- `internal/paths/`, `internal/version/` - install-root paths, build info
- `internal/webui/` - `go:embed` of the built setup page
- `ui/` - the setup page source (React/Vite)
- `scripts/` - `build-webui.sh|ps1` build `ui/` into `internal/webui/assets/`; `smoke/m1.ps1` is a Windows smoke test

`go:embed` cannot reach outside its package, so `scripts/build-webui.sh` builds
`ui/` and copies the result over `internal/webui/assets/`. The built assets are
committed, and CI fails if a fresh build of the UI sources differs from them in any
file (`index.html`, `assets/*` or `bundle.hash`).
