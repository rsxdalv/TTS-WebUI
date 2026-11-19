# supervise: wiring into the launcher

New package `core/internal/serverpy/supervise`, written as a drop-in replacement
for `launcher.Supervisor` as it was at the rings commit (52f85f7 = 9b6d3b5).
Nothing existing was modified. The steps below are for Rob.

## Status: swapped in (LAB-12)

The launcher now supervises the app with `supervise`. `launcher.Supervisor`
(supervisor.go) is still there for one release as a rollback:
**`TTS_GO_SUPERVISOR=legacy`** selects it; unset, `""` or `supervise` selects
this package; any other value makes Start fail with an error. The wiring is
`core/internal/launcher/supervise_swap.go`; `launcher.New(root, opts...)` takes
`launcher.WithNoBrowser()` (from `--no-browser`).

What PR 741 added to the old Supervisor now lives here, through hooks in
`Config` so that `supervise` does not import `launcher`:

- **React front** (`react_ui_backend` on `ReactPort`): `Plan.React` is
  `ReactStatic` (Go serves `react-ui/out`, no Next; the react-ui child is a
  stand-in that lives until Stop), `ReactNextProxied` (Next on a private
  loopback port with `-H 127.0.0.1`, readiness checked on that port, status
  reports the public one) or `ReactNextDirect` (Next owns the port). Switches
  come from `Plan.Switches`. The front closes when Next exits or on Stop.
- **`Config.Prepare`** runs after the GPU check, before anything starts; the
  launcher's version does `switches.FromEnv` + `CheckRoutes`/`CheckSteps`, the
  `TTS_GO_SERVERPY` value check, `sweepOrphans`, `prepareReactUI` (recover and
  build) and reads config.json for the public port and `inbrowser`. It returns
  a `Plan` (React mode, `PublicPort`, `GradioEntry`, `GradioEnv`, `OpenBrowser`).
- **`Config.StartGradioSide`** is where `startServerPy(root, prefix, baseEnv)`
  plugs in (`TTS_GO_SERVERPY=1`): its `Env` goes to gradio only and its `Close`
  runs when gradio exits, on Stop, or when gradio fails to start.
- **`Config.OnStarted(refs)` / `OnStopped()`** write and clear the orphan record
  (`recordGroups` / `clearRecordFor`; Unix only, a no-op on Windows).
- Stop is parallel (already was).

Browser: `Plan.OpenBrowser` is `inbrowser` from config.json and not
`--no-browser`; `Config.SkipReactTab` (set by the launcher) opens only the
public URL. server.py would also open a tab by itself, so the launcher gives
the gradio child `BROWSER=tts-go-noop-browser`, which makes Python's
`webbrowser.open` fail quietly. (The legacy path leaves Python's tab alone.)

Changed outside this package: `launcher.go` (field is now the `appSupervisor`
interface), `go_steps.go` (`appRunning` asks `Running()`),
`orphans_unix.go`/`orphans_windows.go` (`recordGroups`, `clearRecordFor`),
`cmd_launch.go` (`WithNoBrowser`), and one test, `go_steps_internal_test.go`,
which poked `l.sup.procs` and now uses a fake supervisor.

The "Changes in existing files" text below is the original plan; the launcher
differs from it in that the old Supervisor stays and the interface adapts both.
`--public-port`, `--react-port` and the other flags were not added: the public
port comes from config.json.

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

### pid
`procs.Proc.Pid()` exists since PR 741, so the status carries `pid`.

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
