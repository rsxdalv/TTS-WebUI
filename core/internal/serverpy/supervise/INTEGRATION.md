# supervise: wiring into the launcher

New package `core/internal/serverpy/supervise`, a drop-in replacement for
`launcher.Supervisor`. Nothing existing was modified. The steps below are for
Rob.

## Changes in existing files

### `core/internal/launcher/launcher.go`
- Field `sup *Supervisor` becomes `sup *supervise.Supervisor`.
- `New(root)` must build it: `supervise.New(supervise.Config{Root: root, ...})`
  returns `(*Supervisor, error)`. Either make `launcher.New` return an error,
  or pass a `supervise.Config` in (recommended: `New(root, supCfg)`, so
  `cmd_launch.go` controls ports and flags).
- The handlers need no signature changes: `Start(ctx)`, `Stop()`,
  `Restart(ctx)`, `Statuses()`, `SubscribeStatus()`, `AppLink()` have the same
  shapes. `Start` returns `supervise.ErrAlreadyRunning` (text unchanged, so the
  string compare in `handleAppStart` still works; prefer
  `errors.Is(err, supervise.ErrAlreadyRunning)`).
- `/api/app/status` and the SSE stream now carry
  `[{name,running,state,port,pid?,exit_code?,error?,log_tail?}]`. `name` and
  `running` are unchanged, so `LaunchStep.tsx` keeps working.

### `core/internal/launcher/supervisor.go` (and its test)
Delete, with `freePort`, `readGPULabel`, `ProcessStatus`, `appProcess`; the
new package has its own copies. `supervisor_test.go`'s tests move to
`supervise` (already covered there). `logSub` in `log.go` stays (used by Log).

### `core/cmd/tts-webui/cmd_launch.go`
Add flags `--public-port` (7770), `--react-port` (3000), `--no-react`,
`--share`, `--docker`, `--no-tree-proxy` and pass them in `supervise.Config`.
The launcher's own `OpenBrowser` stays for the launcher page; app tabs are
opened by the supervisor when `Config.OpenBrowser` is true.

### Switching to `server_core.py` once stages 2-3 land
```go
supervise.Config{
    Root: root, PublicPort: 7770, ReactPort: 3000,
    GradioEntry: "server_core.py",
    GradioEnv: []string{
        "TTS_CORE_URL=" + coreURL,        // Go control API base URL
        "TTS_CORE_TOKEN=" + token,        // secret: gradio child only
        "TTS_GRADIO_PORT=" + gradioPort,  // port Gradio itself binds
        "TTS_GRADIO_HOST=127.0.0.1",
    },
}
```
`GradioEnv` goes to the gradio child only; `ExtraEnv` goes to all three.
`PublicPort` must be the port the Go router serves, since readiness, AppLink,
the browser tab and React's `GRADIO_BACKEND_AUTOMATIC` all use it.

### Optional: `core/internal/procs/proc.go`
`procs.Proc` has no pid accessor, so `pid` is omitted from the status. Add
```go
func (p *Proc) Pid() int { p.mu.Lock(); defer p.mu.Unlock(); if p.cmd == nil || p.cmd.Process == nil { return 0 }; return p.cmd.Process.Pid }
```
and the supervisor reports it automatically (it type-asserts `Pid() int`).

## Behaviour summary

- Children: `<prefix> python <entry> <args...>`, `<prefix> npm --prefix react-ui start -- -p <ReactPort>`,
  `<prefix> python installer_scripts/tools/extension_manager.py --port <N>`;
  prefix defaults to `micromamba -p installer_files/env run`, cwd = root,
  `MAMBA_ROOT_PREFIX` and `TTS_EXTENSION_MANAGER_PORT` set as before.
- React env: `REACT_UI_PORT`, `GRADIO_BACKEND_AUTOMATIC=http://127.0.0.1:<PublicPort>/`.
  Gradio also gets `REACT_UI_PORT` (unless NoReact). Default gradio args
  `--no-react` (React is the supervisor's job); `--share/--docker/--no-tree-proxy` are appended.
- States: `stopped`, `starting`, `ready`, `stopping`, `exited`, `failed`,
  `disabled` (react with NoReact). `ready` = HTTP GET `/` < 500 on the public
  port (the proxy tree answers 502 until Gradio is up), TCP connect for React
  and the extension manager. `failed` = start error or no readiness within
  `ReadyTimeout` (default 10 min); a timed-out child is left running, not killed.
  `exited` carries `exit_code` and the last 50 log lines. No auto-restart.
- `Start` is refused only while some child is alive, so it works again after a
  crash. The GPU gate (`installer_scripts/.gpu` must parse) is kept; disable with `SkipGPUCheck`.
- Browser: with `OpenBrowser`, the public URL opens when Gradio is ready, then
  the React URL once React is ready (always after Gradio's; skipped if Gradio
  never became ready). Python's own `inbrowser` config can still open a tab from
  server.py; turn that off if you see double tabs.
- `Stop` stops the children in parallel (was sequential, up to 5 s each).
  Subscriber fan-out no longer races with cancel (old code could send on a closed channel).

## Verified

Tests only (`go test ./internal/serverpy/supervise/...`, `go vet` clean, run
3x without flakes): fake child binary from `testdata/fakechild` covering
readiness (including the 502-until-ready HTTP case), timeout, exit code and
log tail, args/env per child, secret isolation, browser order, partial start
failure, restart, context ownership, status JSON shape, AppLink, GPU gate.

Not verified: the real micromamba/Python/npm children, `-race` (needs cgo,
unavailable here), the Windows CTRL_BREAK/job-object stop path beyond what
`procs` itself does, a real browser opening. Known limit inherited from
`procs`: its exit signal waits for the output pipes to close, so if Gradio
dies while a grandchild (extension process) still holds the pipe, `exited`
appears only when that grandchild exits too. A foreign process already
listening on the public port would make readiness report `ready` early.
