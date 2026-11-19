# Go port contracts

Every task in the Go port obeys this file. The originals are the spec for behaviour. This file is the spec for how the
Go pieces fit together. Line references are to commit `52f85f7`.

Two rules apply everywhere:

- **Off by default.** With no switch set, the app runs the old scripts and Next routes exactly as it does today.
- **Windows is the machine under test.** CI only runs Linux, so Windows behaviour must be run here for real.

## 1. Marker files

All markers live directly in `installer_scripts/`. A port writes the same bytes as the original, because the readers
compare raw file content (`initializeApp.js:132-137`, `init_app.js:68`). Do not add a trailing newline unless the table
shows one.

| File | Written by | Exact content | Read by |
|---|---|---|---|
| `.gpu` | `initializeApp.js:146,253`; Go `handleGPU` (`handlers.go:154`) | The label, no newline. One of `NVIDIA GPU`, `AMD GPU (ROCM, Linux only)`, `Apple M Series Chip`, `Intel GPU (XPU)`, `CPU`, `Custom (Manual torch install)` | `initializeApp.js:145,247`, `diagnostic.bat`, `diagnostic_inner.sh`, `readGPULabel` (`supervisor.go:268`, trims) |
| `.preflight_ok` | Go only, `preflight.WriteMarker` (`preflight.go:115`) | `ok\n`. The legacy flow never writes it | Existence only (`steps.go:48`) |
| `.git_version` | `init_app.js:52,114`, last step of a successful install | `git rev-parse HEAD`, trimmed, no newline | `init_app.js:68` (raw compare); existence means install done (`steps.go:67`) |
| `.major_version` | `initializeApp.js:91,142`, after torch installs | `1.0.0`, no newline | `initializeApp.js:141,261` (raw compare), diagnostics |
| `.pip_packages` | `initializeApp.js:144,198,228` | `String(versions.json .pip_packages)`, currently `1.2.0`, no newline | `initializeApp.js:143,185` (raw compare) |
| `.python_version` | `initializeApp.js:33`, shell redirect of `python --version` | Raw shell output, so it ends in `\r\n` on Windows | `initializeApp.js:34` (trimmed), diagnostics |
| `.db_version` | `applyDatabaseConfig.js:49`, only when the file already exists and differs | `1`, no newline. A fresh install never creates it | `applyDatabaseConfig.js:37-39` |

Step completion in the launcher is defined in `steps.go`: download = `installer_scripts/` exists, preflight =
`.preflight_ok`, mamba = the micromamba binary exists, GPU = `.gpu`, install = `.git_version`.

`.git_version` is written last. A port must never write it before the whole flow succeeded, because it is how the
launcher decides the install finished.

## 2. Environment variables

**Python app** (`server.py`, run with the install root as its working directory):

| Variable | Use |
|---|---|
| `REACT_UI_PORT` | Port for the legacy `npm start` path and the browser link (`server.py:31`). Default 3000 |
| `GRADIO_TREE_PORT`, `GRADIO_TREE_URL`, `GRADIO_TREE_HOSTNAME` | Set by `server.py:43-53` itself for the proxy tree. Do not set them from Go |
| `TTS_WEBUI_API_HOST`, `TTS_WEBUI_API_PORT`, `TTS_WEBUI_DB_PATH` | Database API settings (`tts_webui/database/`) |
| `.env`, `.env.user` | Loaded by Python from the install root. `.env` is created by Python when missing. Go never writes either |

**`init_app.js`** reads no environment variable itself. It needs:

- `--silent` in argv, or it opens a browser (`js/server.js:166`).
- The install root as its working directory (`.git`, `requirements.txt`, `react-ui` are relative paths).
- An activated `installer_files/env`, so that `node`, `npm`, `python`, `pip`, `uv`, `git` and `conda` resolve from it.
  The launcher does this with `micromamba -p installer_files/env run` and `MAMBA_ROOT_PREFIX=installer_files/mamba`
  (`handlers.go:191-193`).

**Next** (`react-ui`):

| Variable | Use |
|---|---|
| `GRADIO_BACKEND`, then `GRADIO_BACKEND_AUTOMATIC`, then `http://127.0.0.1:7770/` | Gradio address (`backend-utils/defaultBackend.tsx`) |
| `GRADIO_AUTH` | `user:password` for the Gradio client (`gradio/[name].tsx:49`) |
| `NEXT_PUBLIC_DB_API_URL` | Inlined at build time (`lib/databaseApi.ts:11`) |

`next start` also loads `react-ui/.env.production.local`, `.env.local`, `.env.production` and `.env`, in that order,
with the process environment winning. Go routes that need these values read them through `Deps` (section 5), which
applies the same order, so both paths use the same backend.

**Launcher supervisor** sets `MAMBA_ROOT_PREFIX` and `TTS_EXTENSION_MANAGER_PORT` for app processes
(`supervisor.go:90-91`). Nothing reads `TTS_EXTENSION_MANAGER_PORT` today (see section 9).

## 3. Step runner

Code: `core/installer_scripts/runner.go`. Every port uses it and does not start processes any other way.

```go
// One exported function per original script.
func InitMamba(ctx context.Context, s *Step) (exitCode int, err error)

type Step struct {
    Root   string // absolute install root, the working directory of the originals
    Run    Runner
    Log    LogSink // like displayMessage
    Prompt func(ctx context.Context, label string, choices []string) (int, error) // nil when nobody can answer
}

type Runner interface {
    Run(ctx context.Context, c Command) (Result, error)
}

type Command struct {
    Name  string   // run a program directly, like $ and $$ in js/shell.js
    Args  []string
    Shell string   // or run a line through the shell, like $sh and execCapture
    Dir   string   // default: Step.Root
    Env   []string // KEY=VALUE added on top of the base environment
    Activated bool // run inside installer_files/env
    Capture   bool // return stdout in Result.Stdout instead of logging it
}

type Result struct {
    ExitCode int
    Stdout   string
}
```

Behaviour:

- `exitCode` is what the original would exit with. `err` means the port itself could not continue, and the caller
  treats it as exit 1.
- A non-zero exit from a command is a `Result`, not an `err`. `Result.Check()` gives the error the JS promises reject
  with. `err` is for a command that could not start or was cancelled.
- Output is streamed line by line to `Log`. `\n`, `\r\n` and a lone `\r` each end a line, and a line is delivered as
  soon as it ends, so progress bars show up. Stderr lines get the prefix `[stderr] `, as `procs.Proc` does today.
  Output is always drained, so a child never blocks on a full pipe.
- A `Name` command is finished when its process exits, like the `exit` event `$` and `$$` wait for. Output still in the
  pipes gets up to 2 seconds to arrive, and later output from a background process is dropped.
- A `Shell` command is finished when its output closes, like the `close` event `$sh` and `exec` wait for.
- When a command finishes on Windows, any process it left behind is killed with its job object.
- `Shell` uses `cmd.exe /d /s /c "<line>"` on Windows and `/bin/sh -c` elsewhere, the same as Node's `exec`.
- `Activated` captures the environment of `micromamba -p installer_files/env run` once. Call `Step.ResetActivated`
  after anything that changes that environment's packages.
- Cancelling `ctx` kills the whole process tree.
- If the original asks the user something and `Step.Prompt` is nil, the port logs the reason and exits 1. It never
  guesses an answer. The launcher's GPU step writes `.gpu` first, so the install flow does not need to ask.

## 4. Switches

Code: `core/internal/switches`. All switches are environment variables read by the launcher when it starts the app.

| Variable | Values | Effect |
|---|---|---|
| `TTS_GO_PROXY` | unset, empty or `0` = off, `1` = on | Go listens on port 3000 in front of Next |
| `TTS_GO_ROUTES` | comma list of claim names from section 5, or `all` | Those routes are answered by Go. Needs `TTS_GO_PROXY=1` |
| `TTS_GO_STEPS` | comma list of `mamba`, `install`, `update`, `diagnostic`, or `all` | Those launcher steps run the Go port instead of the script (task I7) |
| `TTS_GO_PROXY_LISTEN` | unset, empty or `loopback` = loopback only, `lan` = every interface | Where the proxy listens (section 6, D18). `lan` needs `TTS_GO_PROXY=1` |
| `TTS_GO_PROXY_ALLOWED_HOSTS` | comma list of host names or IP addresses, without ports | Extra hosts the Host and Origin guards accept in LAN mode, such as a machine name. Needs `TTS_GO_PROXY_LISTEN=lan` |

An invalid value stops the app from starting with a clear error, so a typo can never silently run the other code path.

**Proxy off** means the launcher starts Next with the same arguments as today:
`npm --prefix react-ui start -- -p 3000`. No Go listener exists on port 3000. A unit test must assert these arguments.

**Proxy on** means:

1. The supervisor picks a free loopback port and starts Next with `-- -p <port> -H 127.0.0.1`.
2. Go listens on port 3000 and forwards to that port.
3. If Next exits, Go stops listening on 3000 too, so the status shown to the user stays true.
4. Stop shuts down Go's listener and Next. Nothing is left running.

## 5. Route claims

A claim is a route Go answers instead of Next. Everything not claimed goes to Next unchanged.

Each route registers itself from its own file, so route tasks never edit a shared file. B1 provides this API in
`core/react_ui_backend/claims.go`:

```go
package react_ui_backend

type Deps struct {
    Root          string       // absolute install root
    GradioBackend string       // resolved as in section 2
    GradioAuth    string       // user:password, or empty
    Next          http.Handler // forwards the request to Next
}

type Claim struct {
    Switch   string   // the name used in TTS_GO_ROUTES
    Patterns []string // net/http ServeMux patterns
    Handler  func(Deps) http.Handler
}

func RegisterClaim(c Claim) // call from init() in the route's own file
```

| Switch | Patterns | Go file | Task | Original |
|---|---|---|---|---|
| `webui-generations` | `GET /api/webui-generations/{path...}` | `webui_generations.go` | B2 | `pages/api/webui-generations/[...name].ts` |
| `upload` | `POST /api/upload` | `upload.go` | B3 | `app/api/upload/route.ts` |
| `gradio-file-proxy` | `/api/gradio-file-proxy/{path...}` | `gradio_file_proxy.go` | B4 | `pages/api/gradio-file-proxy/[...path].tsx` |
| `data-outputs` | `GET /api/go-data/outputs/{name}` | `data_outputs.go` | B5a | `getStaticProps` of `pages/outputs/[name].tsx` |
| `data-bark-voices` | `GET /api/go-data/bark-voices` | `data_bark_voices.go` | B5b | `getStaticProps` of `pages/text-to-speech/bark/voices.tsx` |
| `gradio` | `/api/gradio/{name}` | `gradio.go` | B6 | `pages/api/gradio/[name].tsx` |

Everything else passes to Next, including `/`, every page, `/_next/*`, `/_next/data/*` and files from `public/`.

Dispatch order for each request on port 3000:

1. Host and Origin checks (section 6).
2. Forward to Next, before any claim matching, if the path is one Go does not answer:
   - The raw path (`URL.EscapedPath()`) or the decoded path (`URL.Path`) has an empty, `.` or `..` segment. Both are
     tested, so `%2e%2e` counts as `..`.
   - The raw path contains an encoded separator (`%2f` or `%5c`, in either case), or the decoded path contains `\` or a
     NUL byte.
   - The path ends in `/`.
   - The path equals the prefix of a `{path...}` claim without its slash, such as `/api/webui-generations`.
3. If an enabled claim matches the method and path, its handler runs. Go's mux must never answer with its own redirect
   or `405`, so match with `mux.Handler` and serve only a real claim.
4. Otherwise forward to Next. This includes other methods on a claimed path.

So the expected result for every traversal case in section 8 is Next's own response, on both sides of the comparison.

`GET` patterns also match `HEAD`. Path values are untrusted input. A handler that touches the disk must reject anything
that leaves its allowed directory, and must have a test for it.

## 6. Port 3000: binding, auth and origin policy

**Today**, `next start -p 3000` has no `-H`, so Next binds all interfaces (`next` 16.2.1 CLI help: "default: 0.0.0.0").
It has no token, Host check or Origin check. This stays true whenever the proxy is off.

**With the proxy on**, port 3000 follows D9 except for the token, as recorded in D17 and made final in D18:

| Rule | Policy |
|---|---|
| Bind | `127.0.0.1:3000`, plus `[::1]:3000` when IPv6 loopback exists. All interfaces only in LAN mode (below) |
| Next | Binds `127.0.0.1` on its internal port only |
| Host | The request must have host `127.0.0.1`, `localhost` or `[::1]` with port 3000. Otherwise `400` |
| Origin | Checked on every `/api/*` request and every request that is not `GET` or `HEAD`. If an `Origin` header is present it must be `http://127.0.0.1:3000`, `http://localhost:3000` or `http://[::1]:3000`. Otherwise `403`. `Origin: null` is refused |
| Token | None. Pages are served by Next and have no way to receive the launcher's token |
| Launcher API | Never exposed on port 3000. Port 7700 keeps its token. A claim must not start processes or change the install |

**LAN mode** (`TTS_GO_PROXY_LISTEN=lan`, opt-in, D18) changes only these rules:

| Rule | Policy in LAN mode |
|---|---|
| Bind | `0.0.0.0:3000`, plus `[::]:3000` when IPv6 exists |
| Host | Also accepts this machine's own interface addresses and the `TTS_GO_PROXY_ALLOWED_HOSTS` names, with port 3000. Every other name still gets `400`, so DNS rebinding stays blocked. The addresses are reread at most every 5 seconds when an unknown host arrives |
| Origin | Also accepts `http://<an accepted host>:3000` exactly. Other schemes, ports, paths, user info and hosts still get `403` |
| Warning | The launcher logs one line naming the network URLs and saying there is no password |

Next and Gradio stay on loopback in every mode. There is still no password, so LAN mode is only for networks the user
trusts.

Consequences to keep in mind:

- Without LAN mode, the UI cannot be opened from another device while the proxy is on.
- Docker runs `python3 server.py --docker` and never uses the launcher, so it is not affected.
- The internal Next port has no Host or Origin check. It is a random loopback port, so this is no wider than port 3000
  is today.

**Forwarding.** The proxy keeps `Host` and `Origin` unchanged. It sets `X-Forwarded-For`, `X-Forwarded-Host`,
`X-Forwarded-Proto: http` and `X-Forwarded-Port: 3000`, replacing anything the client sent, because Next would otherwise
fill in its internal port. Responses are flushed immediately (`FlushInterval: -1`). There is no limit on body size, and
no read, write or response-header timeout. Gradio generations can run for many minutes before the first byte.
Compression is passed through untouched.

## 7. Page data with the proxy off (B5)

Only two pages read the disk at runtime. The others return constants, so they need no edit and no endpoint:
`/`, `/db`, `/outputs/generations`, `/outputs/voice-drafts` and `/extensions/[name]`.

| Page | Claim | Endpoint response |
|---|---|---|
| `/outputs/[name]` | `data-outputs` | The `outputs` array from `getDataFromJSON(name)` |
| `/text-to-speech/bark/voices` | `data-bark-voices` | The `outputs` array from `getNpzDataSimpleVoices()` |

The page edits must keep the app identical with the proxy off:

- **`getStaticProps` and `getStaticPaths` stay as they are.** They remain the data source while the proxy is off.
- The proxy tells the page which data claims are on with a cookie. On every response to a document request
  (`Sec-Fetch-Dest: document`, or `Accept` containing `text/html`), it sets `tts-go-claims=<names joined by .>` with
  `Path=/; SameSite=Strict` and no expiry. If no `data-*` claim is on and the request carried the cookie, it expires it.
  With the proxy off nothing sets this cookie.
- The page calls this hook, added by B5a in `react-ui/src/data/useGoData.ts` and reused by B5b:

```ts
import { useEffect, useState } from "react";

const claimIsOn = (claim: string) =>
  document.cookie
    .split("; ")
    .find((c) => c.startsWith("tts-go-claims="))
    ?.slice("tts-go-claims=".length)
    .split(".")
    .includes(claim) ?? false;

export function useGoData<T>(claim: string, url: string, initial: T): T {
  const [data, setData] = useState(initial);
  useEffect(() => {
    setData(initial);
    if (!claimIsOn(claim)) return;
    let cancelled = false;
    fetch(url)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (!cancelled && d !== null) setData(d);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [claim, url, initial]);
  return data;
}
```

With the cookie absent, the page makes no extra request and renders the static props. If a fetch fails, the static
props stay on screen.

One case differs from a browser that never saw the proxy. If the proxy is turned off while a browser stays open, that
browser keeps the session cookie until it closes. Each load of these two pages then makes one extra request, which Next
answers with `404`, and the static props stay on screen.

`name` in `/api/go-data/outputs/{name}` must be one path segment that does not start with `.`. Anything else is `404`.

## 8. Side-by-side test protocol

Each route task runs the real Next route and its Go route against the same files and compares them with the T1 harness
in `core/react_ui_backend/compare/`.

**Setup.** Build a scratch install at `C:\scratch\<session>-sbs\`:

- `react-ui\` holds a copy of `.next`, `public`, `package.json` and `next.config.js` from `C:\scratch\baseline\react-ui`,
  and a junction `node_modules` pointing at the baseline's.
- `outputs\`, `favorites\`, `voices\` and the other data folders hold the test fixtures.
- Next runs from that `react-ui\` on a port from your range, and Go runs with `Deps.Root` set to the scratch folder.
- Tests that write files, such as uploads, never run against `C:\scratch\baseline`.
- Hold the `backend` lock only when the test needs the real Gradio on 7770.

**Compare, for each case:** status code; `Content-Type`, `Content-Length`, `Content-Disposition`, `Cache-Control`,
`Accept-Ranges`, `Content-Range`, `Location`; and the body bytes. Dates and other volatile values may only be ignored
through a rule named in the test case, with the reason.

**Every route covers:** the normal case; each error branch of the original; methods it does not handle; path traversal
and encoded traversal (`..`, `%2e%2e`, `%2f`, `%5c`); dotfiles; and missing files.

**Also where relevant:**

- *Large upload:* just under and just over the original's limit, and the memory used by Go during the upload.
- *Streaming:* the first bytes must arrive at the client before the upstream finishes. Measure this; matching bytes at
  the end do not prove streaming.
- *Range requests:* `Range: bytes=0-99`, an open range, and an invalid range.
- *Static pages:* compare with `/_next/data/<BUILD_ID>/...` for a name that was not rendered at build time, because
  build-time pages hold stale data.

**A check must be able to fail.** The harness has a self-test proving that a changed status, header and body are each
reported as differences. Every task then clicks the real UI in a browser with its claim on. Results go in the task's
file, stating what was run and what was not.

## 9. Known problems in the originals

Port these as they are unless a rule below says otherwise. Record any you meet in your task file.

| Where | Problem | What the port does |
|---|---|---|
| `initializeApp.js:317` | `checkIfTorchHasCuda()` is not awaited, so torch is never repaired | Keep the behaviour: no repair |
| `applyDatabaseConfig.js:74` | `createDB` is undefined, so an upgrade fails and is caught at `initializeApp.js:292-297` | Keep the logged failure and carry on. Do not write `.db_version` on that path |
| `js/server.js:160`, `extension_manager.py:143` | Both use port 7771 | Keep |
| `supervisor.go:91,105` | The launcher passes `--port` and `TTS_EXTENSION_MANAGER_PORT`, but `extension_manager.py` reads neither and always uses 7771 | Unchanged by this port |
| `backend-utils/getFile.tsx:11-17` | A file name from the request is read from `public/` with no path check | **Fix in Go:** refuse anything outside `react-ui/public` |
| `pages/outputs/[name].tsx:30` | `name` is used in a path unchecked | **Fix in Go:** section 7 |
