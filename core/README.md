# TTS WebUI core (Go)

Side-by-side rewrite of the app's core: installer/bootstrap, process
supervisor, reverse proxy and static UI host. Engines (Python venvs managed by
uv, native binaries such as audio.cpp) stay out-of-process.

**Rule: pure Go, `CGO_ENABLED=0`, no third-party dependencies unless agreed.**

## Commands (run from `core/`)

```sh
go vet ./...
go test ./...
test -z "$(gofmt -l .)"          # formatting check

go build -o bin/tts-webui ./cmd/tts-webui
./bin/tts-webui --version
```

Release-style build with version info:

```sh
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/rsxdalv/TTS-WebUI/core/internal/version.Version=1.0.0 \
    -X github.com/rsxdalv/TTS-WebUI/core/internal/version.Commit=$(git rev-parse --short HEAD) \
    -X github.com/rsxdalv/TTS-WebUI/core/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o bin/tts-webui ./cmd/tts-webui
```

Cross-compile (bash):

```sh
CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o dist/tts-webui-linux-amd64 ./cmd/tts-webui
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/tts-webui-darwin-arm64 ./cmd/tts-webui
```

In PowerShell set `$env:CGO_ENABLED=0; $env:GOOS="linux"; $env:GOARCH="amd64"` first.

## Layout

- `cmd/tts-webui/` - entry point
- `internal/version/` - build metadata set via `-ldflags -X`
- `internal/webui/` - `go:embed` of the static UI (`assets/`)
- `ui/dist/` - Vite build output (git-ignored except `.gitkeep`)

Embedding: `go:embed` cannot reach outside its package, so the release build
copies `ui/dist/*` over `internal/webui/assets/` before `go build`. The
committed `assets/index.html` is only a placeholder.

Planned `internal/` packages (not created yet):

- bootstrap
- gpu
- env
- engine
- supervisor
- router
- catalog
- outputs
- update
