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
| `TTS_GO_PROXY` | unset, empty or `1` = on (the default since D18), `0` = off | Go owns port 3000 and serves the UI (section 10) |
| `TTS_GO_ROUTES` | unset or empty = every claim (with the proxy on), `none`, `all`, or a comma list of claim names from section 5 | Those routes are answered by Go. A list or `all` with `TTS_GO_PROXY=0` is an error; `none` is allowed. `none` cannot be combined with names |
| `TTS_GO_STEPS` | unset or empty = every step runs its Go port; `none` = every step runs the original script; or a comma list of `mamba`, `install`, `update`, `diagnostic`, or `all` | Which launcher steps run the Go port instead of the script. **On by default since D19 (task C1).** `none` cannot be combined with names |
| `TTS_GO_PROXY_LISTEN` | unset, empty or `loopback` = loopback only, `lan` = every interface | Where the proxy listens (section 6, D20). `lan` with `TTS_GO_PROXY=0` is an error |
| `TTS_GO_PROXY_ALLOWED_HOSTS` | comma list of host names or IP addresses, without ports | Extra hosts the Host and Origin guards accept in LAN mode, such as a machine name. Needs `TTS_GO_PROXY_LISTEN=lan` |

For `TTS_GO_STEPS`, the launcher logs one line at the start of each of those steps saying whether the Go port or the original script ran.

An invalid value stops the app from starting with a clear error, so a typo can never silently run the other code path.

**Proxy off** (`TTS_GO_PROXY=0`, the escape hatch) means the launcher starts Next with the same arguments as before:
`npm --prefix react-ui start -- -p 3000`. No Go listener exists on port 3000. A unit test asserts these arguments. If
`react-ui/.next` is not a usable server build (section 10), or is older than the static export, the supervisor first
runs `npm run build` once in `react-ui`, logs which of the two it is, and only then starts Next. Start waits for that
build. If the build fails, or leaves a build that is still missing or older than the export, Start fails and nothing
starts: the old server build is never served instead.

**Freshness of the two builds (S2F, R2-6).** Each build records the git commit it was built from:
`out/_tts-export.json` has `"commit"` (written by `build:static`), and `.next/tts-build.json` has `"commit"` (written by
the `postbuild` script of `npm run build`, so the legacy installer's build records it too). A build whose commit differs
from the install's `git rev-parse HEAD` is stale. Rebuilding one build therefore never makes the other look stale.
Commits are used instead of file times because two builds of the same commit are equally fresh whichever was built
last, and an Update changes HEAD whichever installer ran.

A build without a recorded commit (made before this rule), or an install where HEAD cannot be read, falls back to file
times. The server build is stale when `.next/BUILD_ID` is older than `out/_tts-export.json`. The export is stale when
`out/_tts-export.json` is older than `.next/BUILD_ID`. `next build` writes `BUILD_ID` anew. `build:static` only renames
`.next` aside and back, which keeps the file's time, so a restored old build still counts as old (a unit test checks
the rename).

- **A stale server build** is rebuilt once with `npm run build` before Next starts, in the escape hatch and in mode 2
  below when the export exists but its claims are off.
- **A stale export** is rebuilt once with `npm run build:static` before static mode serves it, with a log line saying it
  comes from another commit (for example after an Update with `TTS_GO_STEPS=none`, which rebuilds only the server
  build).
- Either rebuild that fails, or leaves its build missing or still stale, fails Start and starts nothing. The old build
  is never served instead.

**Proxy on** (the default) means the supervisor picks one of three modes when the app starts (D18):

1. **Static:** `react-ui/out/_tts-export.json` exists and every claim the export needs is on (`gradio`,
   `gradio-file-proxy`, `upload`, `webui-generations`, `data-outputs`, `data-bark-voices`). Go serves the export on
   port 3000 (section 10) and no Next process runs. The launcher status shows `react-ui` as running while Go serves
   it. If `TTS_GO_ROUTES` turns any of those claims off, generation and upload could not work, so the supervisor uses
   mode 2 when a usable server build exists, with a log line naming the missing claims. Otherwise Start fails with an
   error naming them.
2. **Next behind the proxy:** no export, but `react-ui/.next` is a usable server build (an install made by the old
   scripts). The supervisor logs one line saying so, picks a free loopback port, starts Next with
   `-- -p <port> -H 127.0.0.1`, and Go forwards to it as B1 did. If Next exits, Go stops listening on 3000 too.
3. **Neither:** Start fails with `the React UI is not built: run Install or Update in the launcher`, and nothing starts.

If `react-ui/.next-server-saved` exists (an interrupted export build), the supervisor first runs
`node build-static.js --recover` and decides again. In every mode Stop shuts down Go's listener and any Next process.

Every switch (`TTS_GO_ROUTES` names, `TTS_GO_STEPS` names and the rest) is checked before the mode choice, so a typo
fails Start before any build runs (R2-5).

When there is no export, the mode 2 log line suggests Update only when `react-ui/package.json` has `build:static`.
Otherwise it says that this UI version has no static export and is served through `next start` (R2-4).

**Process lifetime (R2-1).** On SIGINT (Ctrl+C), SIGTERM and SIGHUP (closed terminal), the launcher stops the app (all
processes in parallel: graceful signal to each process group, kill after 5 s) and any running install step, then
exits. It waits at most 20 s.
- SIGKILL cannot be caught. On Linux and macOS, Start records each app process group in
  `installer_files/app-processes.json` (pgid and, on Linux, the leader's start time from `/proc/<pid>/stat`), and a
  clean Stop deletes it.
- When a launcher starts, and when the app starts, it reads a leftover record and stops what is left of those groups,
  logging one line. On Linux it signals only processes that are in a recorded group, started no earlier than the
  recorded leader, and have the install root as working directory, so a reused PID is never killed. On macOS (not run
  by the port's tests) it uses the group and the command line.
- On Linux each child also gets `Pdeathsig` SIGTERM, which `micromamba run` passes on, so a killed launcher usually
  takes the app with it before any sweep is needed.
- On Windows job objects already end everything with the launcher, and nothing is recorded.

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

**With the proxy on**, port 3000 follows D9 except for the token, as recorded in D17 and made final in D20:

| Rule | Policy |
|---|---|
| Bind | `127.0.0.1:3000`, plus `[::1]:3000` when IPv6 loopback exists. All interfaces only in LAN mode (below) |
| Next | Binds `127.0.0.1` on its internal port only |
| Host | The request must have host `127.0.0.1`, `localhost` or `[::1]` with port 3000. Otherwise `400` |
| Origin | Checked on every `/api/*` request and every request that is not `GET` or `HEAD`. If an `Origin` header is present it must be `http://127.0.0.1:3000`, `http://localhost:3000` or `http://[::1]:3000`. Otherwise `403`. `Origin: null` is refused |
| Token | None. Pages are served by Next and have no way to receive the launcher's token |
| Launcher API | Never exposed on port 3000. Port 7700 keeps its token. A claim must not start processes or change the install |

**LAN mode** (`TTS_GO_PROXY_LISTEN=lan`, opt-in, D20) changes only these rules:

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

In a static export the cookie is not used: see section 10.

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

## 10. Static export (S1)

`npm run build:static` in `react-ui` builds the UI as a static export, next to the server build. For S2 (Go serves it) and C2 (the install builds it).

### Build

- `npm run build` and `npm start` are unchanged and still produce and run the server build in `react-ui/.next`.
- `build-static.js` runs `generate-plugin-imports.js` (the `prebuild` step), then `next build` with `TTS_STATIC_EXPORT=1`, which makes `next.config.js` set `output: "export"`. The API routes (`pages/api/*`, `app/api/upload`) are left out by Next itself in export mode (it prints a warning); Go serves them.
- Next 16 always compiles an export in `.next`, so the script sets the server build aside as `.next-server-saved` for the duration and puts it back at the end, also when the build fails.
- **Nothing detects a running `next start`.** Renaming `.next` works while Next serves from it, and the running server then answers 500/400 until the build is back. The caller (C2's update step, S2's migration mode) must stop Next first.
- **A killed export** (power loss, job-object kill; the `finally` does not run) leaves `.next-server-saved` (the real server build), the marker file `.next-export-scratch`, and in `.next` the export's scratch, which may or may not have a `BUILD_ID` and never works with `next start`. `node build-static.js --recover` puts the saved build back (it trusts the marker, not the contents of `.next`). `npm run build:static` does this first, and so does `npm run build` through its `prebuild` script, so a server build made after a kill starts from a restored tree and overwrites it. Rule for anyone deciding whether a usable server build exists (S2): if `.next-server-saved` exists, run `node build-static.js --recover` first, then check `.next`. "Usable" means `.next/BUILD_ID` **and** `.next/prerender-manifest.json` exist: an export killed on a first install (nothing to set aside, so no marker) leaves a `.next` with a `BUILD_ID` that `next start` cannot use. `.next-export-scratch` is git-ignored.
- The export is built in `react-ui/.out-building` and renamed to `react-ui/out` only when it is complete, so a failed build leaves the previous `out/` untouched. `out/_tts-export.json` (`{"buildId": ..., "builtAt": ...}`) exists only in a complete export: its absence means there is no usable export. Both `.out-building` and `.next-server-saved` are git-ignored.
- Needs `src/extensions/link.ts` like the server build (written by the prebuild step). An extension installed later needs a rebuild to be included, except for its page (see the shell below).
- The server build is unchanged by this task: HTML and `_next/data` JSON of 16 pages and 9 data URLs are identical to the ones before it (script and chunk names aside), and so are the cache headers.

### Who builds it (C2)

- The Go install and update steps (`TTS_GO_STEPS` install, update) run `cd react-ui && npm run build:static` where the original runs `npm run build`. A tree whose `react-ui/package.json` has no `build:static` script (main, and so every fresh Download, until S1 reaches main) gets `npm run build` instead, after the log line "react-ui has no build:static script (this tree predates the static export), so building the server build". They build only when the original builds: on install, and on update when `git pull` brings a commit that differs from `.git_version`.
- The legacy JS installer (`init_app.js`) is unchanged and still builds the server build only.
- Because the export sets `.next` aside, the launcher answers `POST /api/steps/install` and `POST /api/steps/update` with `409` and "stop the app before running <step>" while any app process runs, when that step runs its Go port. The script path has no such check.

### Where things are

| In `react-ui/out/` | Contents |
|---|---|
| `<page>.html`, `<dir>/<page>.html` | One file per page, e.g. `text-to-speech/kokoro.html`, `outputs/favorites.html`; `index.html` is `/` |
| `404.html` | The page for an unknown URL |
| `_next/static/**` | Hashed JS, CSS and fonts |
| `_next/data/<BUILD_ID>/<page>.json` | `getStaticProps` data of the 9 SSG pages, `{"pageProps":...,"__N_SSG":true}` |
| `_next/static/<BUILD_ID>/` | `_ssgManifest.js` and `_buildManifest.js` |
| `_not-found.html`, `_not-found/*.txt`, `tools/_api_config.html` | App-router and extra pages; served by rule 4, nothing depends on them |
| `favicon.ico` | From `public/` |
| `extensions/_shell.html`, `_next/data/<BUILD_ID>/extensions/_shell.json` | The extension shell |

`public/file-input-cache` is not copied into the export. Go serves everything in `react-ui/public` live (uploads land there), and the export has no copy of it.

### URL to file (what Go must do)

For `GET`/`HEAD` of a decoded path `P`, in this order, first match wins:

1. Refuse with `404` any path with a segment starting with `.`, a backslash or a NUL (as for `public/`).
2. `P` under `/file-input-cache/` (and any other file in `react-ui/public` that the export lacks): the live file from `react-ui/public`.
3. `P == "/"`: `index.html`.
4. Otherwise strip trailing slashes from `P` (Next answers `/x/` with a `308` to `/x`; either that or serving the page is fine, and the page works under both) and try: the file `P` as is (`/_next/static/...`, `/_next/data/...`, `/favicon.ico`), then `P.html`, then `P/index.html`. `/tools` is `tools.html` while `/tools/gpu_info` is `tools/gpu_info.html`: both exist.
5. `P` matches `/extensions/<one segment>` and nothing above matched: `extensions/_shell.html` with status `200`.
6. `P` matches `/_next/data/<BUILD_ID>/extensions/<one segment>.json` and nothing above matched: that directory's `_shell.json`, status `200`. (Without this, client-side navigation to an extension that was not exported falls back to a full page load; with it, it stays in the page.)
7. Anything else: `404.html` with status `404`.

Route claims (`/api/gradio/*`, `/api/gradio-file-proxy/*`, `/api/webui-generations/*`, `/api/upload`, `/api/go-data/*`) are matched before any of this. A request for `/api/...` that no claim handles is `404`, not a page.

Only the extension shell needs a fallback. The other dynamic page, `/outputs/[name]`, is exported for `outputs` and `favorites` only (export mode uses `fallback: false`); `/outputs/<other name>` is `404`. The UI links only to those two.

### The extension shell

`/extensions/[name]` is exported for `extension_xtts_simple` and `_shell`. The shell page reads the extension name from the URL in the browser (`router.asPath`) after hydration, so one file serves any name, including names of extensions installed after the build. The name is URL-decoded. A name that has no component in `src/extensions/link.ts` renders an empty page with the name as title, as the server build does. `/extensions/huggingface_cache_manager` is its own page and wins over the shell (rule 4).

### Data pages

`/outputs/[name]` and `/text-to-speech/bark/voices` are exported with empty data, so no files of the build machine end up in the export. In an export build `useGoData` skips the cookie check and always asks `GET /api/go-data/outputs/{name}` and `GET /api/go-data/bark-voices`. So an export needs the claims `data-outputs` and `data-bark-voices` on: with either off, that page shows nothing and the extra request is answered `404`. No `tts-go-claims` cookie is needed for an export. `/`, `/db`, `/outputs/generations` and `/outputs/voice-drafts` have constant props.

### Headers (as Next 16 sends them for the same files)

| Files | `Cache-Control` | Notes |
|---|---|---|
| `/_next/static/**` | `public, max-age=31536000, immutable` | Content type by extension (`application/javascript; charset=UTF-8`, `text/css; charset=UTF-8`); weak `ETag` and `Last-Modified` as for any static file |
| HTML pages and `/_next/data/**` | `s-maxage=31536000` | HTML `text/html; charset=utf-8`, data `application/json; charset=utf-8`. Next adds a strong `ETag` and answers `If-None-Match` with `304` |
| `404.html` served for an unknown URL | `private, no-cache, no-store, max-age=0, must-revalidate` | Status `404` |
| Other files of `out/` (`favicon.ico`) and `public/` | `public, max-age=0` | Weak `ETag`, `Last-Modified`, `Accept-Ranges` as for `public/` today |

`s-maxage` is honoured only by shared caches, so a browser revalidates the HTML each load; keeping that header is optional but harmless.

### How Go serves it (S2)

`core/react_ui_backend/static_export.go` implements the rules above, behind the Host and Origin guards and the route
claims (section 5), in this order:
- An `/api/...` path no claim answered gets a plain `404` for any method, never a page.
- Other paths: only `GET` and `HEAD` get files; other methods get `405`.
- Rule 1 also refuses empty segments and the encoded separators `%2f`, `%5c` and `%00`. On Windows it also refuses a
  segment containing `:` or ending in a space or dot (the same hardening as B2).
- On Windows a file that was found is refused when its real (long) name has a part starting with `.`, because NTFS
  8.3 aliases such as `HIDDEN~1` for `.hidden` pass the URL check. B2's webui-generations claim applies the same check.
- A trailing slash gets a `308` to the path without it. Because empty segments are refused, the target is always a
  local path.
- `/file-input-cache/*` comes from `react-ui/public` live. Then come export files, `P.html` and `P/index.html`, then any
  other file in `react-ui/public`, then the extension shells, then `404.html`.
- Pages and data JSON get Next's strong `ETag`, a `304` for a matching `If-None-Match`, and gzip or deflate from 1 KB
  up. Other files are sent by B2's `send` port: weak `ETag`, `Last-Modified`, ranges. `/_next/static` files are marked
  immutable.

### Checked

`core/react_ui_backend/compare/browser/static-export-check.js` loads every page of an export in Edge through a Go proxy in front of `compare/browser/staticsrv.js` (a small reference implementation of the rules above), and follows links to check that client navigation does not reload the page. `staticsrv.js` is a test tool; the real server is S2's.
