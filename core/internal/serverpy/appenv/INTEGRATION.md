# appenv: integration (Stage 2)

Package `github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv`. New files
only; nothing existing was touched. Go stdlib only.

## What it replaces

| server.py / Python                                   | Go                                        |
|------------------------------------------------------|-------------------------------------------|
| `create_output_folders()`                            | `EnsureFolders(root)`                     |
| `init()`: write `.env` when missing                  | `EnsureEnv(root, baseEnv)`                |
| `load_dotenv()` then `load_dotenv(".env.user", override=True)` | `LoadEnv(root, baseEnv)` / `LoadEnvReport` |
| `"--x" in argv`                                      | `ParseFlags(args)`                        |
| `start_gradio_server` port/host logic, `REACT_UI_PORT`, `server_hypervisor` backend URL | `Plan(root, mergedEnv, flags)` |
| `upgrade_gradio_options` auth                        | `PortPlan.Auth`                           |

## What Rob must change (existing files)

In the launcher (`core/internal/launcher/supervisor.go` or wherever the
children are configured), before any child starts, with `root` = the repo root
(the cwd the children run in):

```go
base := os.Environ()
_ = appenv.EnsureFolders(root)
_, _ = appenv.EnsureEnv(root, base)          // before LoadEnv; reads HF_HOME etc. from base
rep, err := appenv.LoadEnvReport(root, base)  // rep.Env = merged env; log rep.Warnings
plan, err := appenv.Plan(root, rep.Env, appenv.ParseFlags(args))
```

Then:

1. **Children's environment** = `rep.Env` + `plan.Env()` for the gradio child
   (`server_core.py` / `server.py`), and `rep.Env` + `plan.ReactEnv()` for
   react-ui. The launcher's env must not be replaced by `plan.Env()` alone.
   `plan.Env()` is `GRADIO_TREE_PORT`, `GRADIO_TREE_URL=` (empty, deliberately),
   `TTS_GRADIO_PORT`, `TTS_GRADIO_HOST`; the tree vars are present only when
   Python would set them. `TTS_CORE_URL` / `TTS_CORE_TOKEN` are the other
   stages' business.
2. **Stage 1 supervisor config**: gradio entry stays `server_core.py --no-react`
   (plus `--share/--docker/--no-tree-proxy` forwarded as given: Python still
   reads them, and Python re-derives the same ports, so Go and Python agree).
   Start react-ui only if `!plan.NoReact`, on `plan.ReactPort`
   (`npm --prefix react-ui start -- -p <ReactPort>`), env `plan.ReactEnv()`.
3. **Stage 3 router**: listen on `plan.Host:plan.PublicPort` (docker: 0.0.0.0:7770)
   and proxy `/` to `http://plan.Host:plan.GradioPort`. With `plan.TreeProxy ==
   false` (`--no-tree-proxy`) gradio itself is on `PublicPort`; there is nothing
   for Go to put in front of it (see quirks for the 7769 tree).
4. **Browser opening**, if the launcher takes it over from Python: open
   `plan.BrowserURLs` when `plan.InBrowser`.
5. **Auth**: `plan.Auth.Credentials` (user + password). Printing any of these
   types (`%v`, `%+v`, `%#v`, `String()`) redacts the password; read
   `.Password` explicitly only where it is needed. Never put it in env or logs.

## Which `.env` is loaded

`load_dotenv()` without a path calls `find_dotenv()`, which starts at the
directory of the *calling file*, `tts_webui/dotenv_manager/init.py`, and walks
up: `tts_webui/dotenv_manager/.env`, `tts_webui/.env`, `<root>/.env`, then the
parents of root. In practice that is **`<root>/.env`** (which `init()` creates
first). A stray `.env` in `tts_webui/` or `tts_webui/dotenv_manager/` shadows it,
and `FindDotenv(root)` reproduces that (tested). `.env.user` is cwd-relative:
`<root>/.env.user`.

## Behaviour notes / quirks reproduced on purpose

- `.env` never overrides variables already in base; `.env.user` does.
  `${VAR}` / `${VAR:-default}` interpolation, including the order rules
  (`.env`: base wins over earlier file keys; `.env.user`: file keys win).
- Windows: Python's `os.environ` upper-cases names, so a `.env` key
  `hf_home` is skipped when `HF_HOME` is in base, and `${Path}` does not
  find `PATH` (only `${PATH}` does). Go folds case on Windows only
  (`runtime.GOOS`); the golden tests force the Windows behaviour.
- Files are read with universal newlines (CRLF and lone CR become LF) and must
  be valid UTF-8 (Python raises otherwise; `LoadEnv` returns an error). A BOM is
  not stripped (Python uses plain utf-8), so it ends up in the first key.
- Names containing `=` or NUL, or values containing NUL, are an error (Python's
  `os.environ` raises).
- Unparsable statements are skipped; `LoadEnvReport` gives their start lines.
- `EnsureEnv` writes CRLF on Windows (Python text mode) and LF elsewhere, UTF-8
  always. Python used the locale encoding (cp1252) and would then fail to read
  non-ASCII model paths back as UTF-8; Go does not have that bug.
- **No config merge.** `load_config.py` returns a present `config.json` as is;
  defaults apply only when the file is absent. `LoadConfig` errors
  (`*ConfigError`) where Python would crash with KeyError/ValueError:
  missing `gradio_interface_options`, `auth`, `server_name` or `server_port`,
  or a non-numeric port. It does not create `config.json`; Python does that when
  it starts. Two small deviations: ports outside 1..65535 are rejected, and a
  null/empty `server_name` becomes `127.0.0.1`.
- Modes (`start_gradio_server`; verified against the real code, below):
  - default: Public = config port (7770), Gradio = 7767, tree env set.
  - `--no-tree-proxy`: Public = Gradio = config port, no tree env.
    Quirk: `setup_gradio_proxy_tree` still runs, so Python starts its tree on
    `GRADIO_TREE_PORT` (default 7769) routing `/` to gradio; `plan.TreePort`
    reports it. Gradio also keeps `inbrowser` (`GradioInBrowser`), so Python
    can open the browser twice.
  - `--docker`: host 0.0.0.0, Public 7770, Gradio 7767, whatever the config
    port, and also with `--no-tree-proxy` (then `TreeProxy` is still true).
    Browser URL under docker + no-tree-proxy is `localhost:7767`.
  - `--share` forces share; `inbrowser` is forced off for gradio in tree mode.
- **Legacy ordering**: `server_hypervisor()` runs before ports change, so React's
  `GRADIO_BACKEND_AUTOMATIC` is `http://127.0.0.1:<config port>/` (`ReactBackend`):
  the public port normally, but under `--docker` with a non-7770 config port
  it points at the wrong port, as today.
- `REACT_UI_PORT` comes from the merged env (default 3000); invalid values are
  an error unless `--no-react`.

## Verified for real vs only in tests

Real Python, captured once by `testdata/gen_golden.py` into `testdata/golden.json`
(python 3.10.11, python-dotenv 1.0.0, Windows, from the repo's installed env;
file hashes of the real sources are recorded in the JSON):

- 19 dotenv cases through the real `tts_webui/dotenv_manager/init.py init()`
  (comments, export, quotes, escapes, inline comments, multiline, CRLF/CR,
  unicode whitespace, BOM, duplicates, interpolation, base/override order,
  Windows case folding, stray `.env` shadowing, unparsable lines): full resulting
  environment and warning line numbers compared.
- 4 `.env` generation cases: byte-exact against what `write_env(generate_env())`
  wrote (CRLF).
- 224 runs of the real `server.py` (`start()`, with gradio/proxy-tree/React/
  browser stubbed to record): 7 config/env variants × all 32 flag combinations;
  compared: gradio port/host/share/inbrowser, tree port and env, browser URLs,
  React command, port and backend URL.

Only in tests (hand-written, no Python): POSIX case-sensitive semantics,
`EnsureFolders`, `LoadConfig` error paths, auth parsing/redaction, base not
mutated. Not exercised against a live launcher or live Gradio. POSIX (case
sensitivity) was never compared with real Python, as only Windows Python exists
here. Regenerate goldens with:

```
installer_files\env\python.exe core\internal\serverpy\appenv\testdata\gen_golden.py <repo> core\internal\serverpy\appenv\testdata\golden.json
```
(it writes only to a temp dir and the output path).

## Wired in (LAB-13.2, `TTS_GO_SERVERPY=1`)

`launcher/serverpy.go` `startServerPy` now does, before any child starts:
`EnsureFolders` -> `EnsureEnv(root, baseEnv)` (logs when it creates `.env`) ->
`LoadEnvReport` -> `Plan`. The merged environment (`rep.Env`: launcher env +
`.env` + `.env.user`) is the extprocs `BaseEnv` and the start of the gradio
child's env (`serverPy.gradioEnv`, followed by `TTS_CORE_*`/`TTS_GRADIO_*`).

On the Python side `server_core.py` installs the bridges and calls
`tts_webui.core_server.main()` instead of running `server.py`. `core_server`
does only: `dotenv_manager.init.init()` (harmless after Go; it re-loads the
same files), torch-load patch, `suppress_warnings`, config, the auth/legacy
option clean-up from `upgrade_gradio_options`, `main_block`, and
`demo.queue().launch(...)` on `TTS_GRADIO_HOST:TTS_GRADIO_PORT` with
`inbrowser=False`. No proxy tree, React, database or browser. `server.py` is
unchanged (legacy path).

Verified: a `.env` generated by Go is byte-identical to `write_env(generate_env())`.
Remaining stray output: `Gradio Proxy Tree initialized on ...:7769` is printed
when `core_router_bridge` imports `gradio_proxy_tree.main` to patch it; nothing
is started.
