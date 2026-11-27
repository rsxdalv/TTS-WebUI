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
go install github.com/air-verse/air@latest   # once
cd core && air

# Setup page (core/ui): hot reload against that launcher.
cd core/ui && npm run dev                    # open http://127.0.0.1:5173/
```

Air needs Go 1.27 first on PATH (`go version`); an old Go fails with "cannot find module providing package cmp" and rewrites `go.mod`/`go.sum`.

`npm run dev` proxies `/api` to `LAUNCHER_URL` (default `http://127.0.0.1:7700`),
fixes the Host and Origin headers and adds the launcher's per-run token, which it reads
from the launcher's index page. After a launcher restart the first request may get a
403; reload the page. The built setup page that `go:embed` serves is still
`scripts/build-webui.sh` (commit its output).

The React UI has its own dev server: `cd frontend && npm run dev` (port 3002, proxies
`/api` to the Go server on 3000, or `VITE_API_PROXY`).

## Layout

- `cmd/tts-webui/` - entry point (`launch`, the default, and `mcp`)
- `internal/mcp/` - the MCP server: two tools over the OpenAI-compatible API ([spec](../documentation/core-rewrite/MCP.md))
- `internal/launcher/` - the setup steps and the app supervisor
- `react_ui_backend/` - the Go server for the React UI on port 3000: serves the static export (`frontend/out`, built by Vite with `npm run build:static`) and answers the API routes (`TTS_GO_ROUTES` picks which; `TTS_GO_PROXY=0` is an error now that Next.js is gone)
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
