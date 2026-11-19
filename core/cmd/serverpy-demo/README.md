# serverpy-demo

A test harness that runs the four `core/internal/serverpy` stages together, without the launcher:

| Stage | Package | What it does in the demo |
|---|---|---|
| 2 | `appenv` | creates `outputs/` and `favorites/`, writes `.env` if missing, loads `.env` + `.env.user`, prints the legacy port plan |
| 3 | `router` | Go reverse proxy on `-public-port`, routing `/` to Gradio on a private port. The Python proxy tree stays off |
| 4 | `extprocs` | starts proxy extensions when the Gradio UI asks, and routes `/<pkg>` to them |
| 1 | `supervise` | runs `server_core.py` (Gradio), the extension manager and optionally React, then reports their states |

Both control APIs (`/api/routes`, `/api/extensions`) are served on a random loopback port, together with an
unauthenticated `GET /status` that shows processes, extensions and routes.

It does not touch the launcher (`core/internal/launcher`, `core/cmd/tts-webui`). The launcher wiring is in each
package's `INTEGRATION.md`.

## Run

From `core/` in this worktree. The env comes from an existing install, because a worktree has no `installer_files/`:

```
go build -o ../serverpy-demo.exe ./cmd/serverpy-demo
../serverpy-demo.exe -env-root C:/Users/admin/Desktop/tts-webui-golang
```

Then open http://127.0.0.1:17770/ (it opens by itself unless `-browser=false`). Ctrl+C stops everything: extensions,
then the supervised children (job-object tree kill), then the router.

| Flag | Default | |
|---|---|---|
| `-root` | `..` | repo root holding `server_core.py`; the Python children's cwd |
| `-env-root` | `-root` | install holding `installer_files/mamba` and `installer_files/env` |
| `-public-port` | 17770 | the router's port. Kept clear of 7770 so a running app is not disturbed |
| `-react`, `-react-port` | off, 13000 | React needs `react-ui/node_modules` and a build in `-root` |
| `-browser` | true | open the public URL once Gradio is ready |
| `-share` | false | forwarded to Gradio |
| `-logs` | true | print the children's output, prefixed `[gradio]` and so on |
| `-exit-after` | 0 | stop after a duration (for unattended smoke runs) |

## Known limits seen in a real run

- The extension manager still opens its own browser tab (`inbrowser=True` in `extension_manager.py`). Set
  `BROWSER` to a command that does not exist (for example `BROWSER=none`) to suppress it during smoke runs.
  On this branch it uses the port the supervisor gives it (`--port`, then `TTS_EXTENSION_MANAGER_PORT`, then 7771).
- The supervisor's readiness probe goes through the router, so the router logs "upstream unavailable" until Gradio is
  up. The demo shows that message once per 15 s.
- Proxy extensions (`"proxy": "native"` in `extensions.json`) run from `.venvs/<pkg>`. If none is installed, the
  extension path can't be clicked through. `extprocs`' opt-in `TestRealExtension` covers it with the main env.
