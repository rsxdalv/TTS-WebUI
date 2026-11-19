# TTS WebUI core (Go)

A single exe that sets up and runs TTS WebUI: it serves a local setup page,
runs the install steps (download, pre-flight, micromamba, GPU choice, install),
then starts and stops the app.

**Rule: pure Go, `CGO_ENABLED=0`, no third-party dependencies.**

## Run

```sh
tts-webui launch            # serves http://127.0.0.1:7700, uses the exe's folder as the install root
tts-webui launch --root D:\tts --port 7700 --no-browser
tts-webui --version
```

## Build and test (from `core/`)

```sh
go vet ./...
go test ./...
test -z "$(gofmt -l .)"
CGO_ENABLED=0 go build -trimpath -o bin/tts-webui ./cmd/tts-webui
```

## Layout

- `cmd/tts-webui/` - entry point (the exe only launches; no other commands)
- `internal/launcher/` - the setup steps and the app supervisor
- `react_ui_backend/` - the optional Go proxy in front of Next.js on port 3000 (off unless `TTS_GO_PROXY=1`)
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
committed, and CI fails if `bundle.hash` no longer matches the UI sources.
