---
applyTo: "core/**"
description: Rules for the Go core (side-by-side rewrite of the TTS WebUI launcher, supervisor and router).
---

# Go core rules

Read `documentation/core-rewrite/PLAN.md` before changing anything under `core/`.

## Hard constraints

- Pure Go. `CGO_ENABLED=0` must always build for windows, linux and darwin on amd64 and arm64. No cgo, no C toolchains.
- No ML code in the core. Inference always runs in engine processes (Python venvs via uv, native binaries such as audio.cpp). The core spawns, supervises, proxies and stores files.
- Stdlib first. A third-party module needs a one-line justification in the PR description, and a note in `documentation/core-rewrite/DECISIONS.md`. Pre-approved: `golang.org/x/sys` (Windows Job Objects, signals), `golang.org/x/sync`.
- The legacy app (`server.py`, `tts_webui/`, `installer_scripts/`, `react-ui/`) must stay runnable. The core never writes into legacy-owned paths unless the current checkpoint says so.

## Layout

```
core/cmd/tts-webui/      main; subcommands only wire packages together
core/internal/<pkg>/     one package per isolable structure (see PLAN.md)
core/internal/webui/     go:embed of the built UI
```

Each package exposes a small interface and is usable on its own through a CLI subcommand
(for example `tts-webui gpu detect`, `tts-webui env create`, `tts-webui engine start <id>`).
That subcommand is how a checkpoint is demonstrated and how parity with the legacy flow is checked.

## Style

- Errors: wrap with `fmt.Errorf("doing x: %w", err)`; no panics outside `main`.
- Context: every blocking or long operation takes a `context.Context` and honours cancellation.
  Cancellation is a product feature here (stopping generations, aborting installs), not an afterthought.
- Logging: `log/slog`. Engine process output is captured per engine and streamed, never just printed.
- Paths: build with `filepath`; never assume `/`. Test on Windows path shapes (`C:\...`, spaces, long paths).
- Processes: on Windows, put every child in a Job Object with kill-on-close. On Unix, use process groups.
  Never leave orphans; a test should prove it.
- Ports: ask the OS for a free port; never use counters or fixed ranges except the documented public port.
- Config and state files: JSON, versioned with a `"schema"` field, written atomically (write temp, rename).

## Tests

- `go test ./...` must pass on Windows, Linux and macOS in CI (`.github/workflows/core.yaml`).
- Table-driven unit tests per package. Anything that downloads or spawns real processes goes behind
  the `integration` build tag and is documented in the package README.
- Use fake engines (a tiny Go test binary or `httptest` server) for supervisor and router tests,
  not real Python.
