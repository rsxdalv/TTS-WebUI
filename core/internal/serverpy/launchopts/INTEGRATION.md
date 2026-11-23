# launchopts: integration

Package `core/internal/serverpy/launchopts`. Go computes the keyword arguments
of Gradio's `launch()` and the lines printed before it; Python
(`tts_webui/core_server.py`) calls `bridge.launch.options()` and then
`demo.queue().launch(**kwargs)`.

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

`GET /api/launch/options` -> `{"options": {...}, "lines": [...]}`. A config that
`appenv.ParseConfig` refuses (the launcher checked it before it started the app)
is a 500 with the reason; nothing is launched.

## What moved

| was | now |
|---|---|
| `core_server.upgrade_gradio_options` (auth to a tuple, drop old keys) | `Compute` |
| `core_router_bridge.gradio_overrides` (`TTS_GRADIO_HOST/PORT`, `inbrowser=False`) | `Compute`; the two variables are no longer set |
| fixed `allowed_paths`, `favicon_path`, `prevent_thread_lock` in `core_server.launch_options` | `Compute` |
| `print_gradio_options` | `describe.go`; Python prints `lines` |
| `--share` (ignored by `core_server`; `server.py` forced `share`) | `Compute`, from `appenv.Flags` |

`Compute` reads `config.json` through `appconfig.Store.Load` (the same mutex as
every write), so what Gradio gets is the file as it is at launch.

- **Private address:** `server_name` and `server_port` are Gradio's own
  (`127.0.0.1` and the free port the launcher chose), not the config's, which
  describe the router's public side. `inbrowser` is false (the launcher opens
  the browser). `--docker` therefore does not change Gradio's options: the router
  binds `0.0.0.0` (`appenv.Plan`) and Gradio stays on loopback.
- **`auth`:** `null`, or a list of `[user, password]` pairs. A string is split
  on `:` and the first two parts are used (Gradio did the same with the longer
  tuple the old code built); a pair is one login; a list of pairs is several (the
  old code built a tuple of lists for those, which Gradio cannot use). Python
  turns the pairs into tuples (JSON has none). A bad `auth` is an error, not a
  silent no-login.
- **Dropped keys:** `file_directories`, `favicon_path`, `show_tips`,
  `enable_queue`, `prevent_thread_lock` of the file; the last two are then set
  to the fixed values below. Fixed: `allowed_paths: ["."]`, `favicon_path:
  ./react-ui/public/favicon.ico`, `prevent_thread_lock: true`.
- **Flags:** the launcher passes its flags (`NoReact` always; `--share` and
  `--docker` from `tts-webui launch`, via `launcher.WithShare`/`WithDocker`; the
  same flags reach `appenv.Plan` and the gradio child's arguments). With
  `Share` the option `share` is true and the line `Gradio share mode enabled` is
  printed; `share` from config stays as written.

## Secrets

`options` carries what Gradio needs, including `auth` and
`ssl_keyfile_password`, over the loopback API with the token. `lines` never
does: `auth` is in none of the printed groups (as before) and
`ssl_keyfile_password` is printed as `"******"` when set. (The old listing
printed the first 17 characters of it.) The server logs nothing from the
options, only the error text of a refused config, which names keys. The legacy
`print_gradio_options.py` (used by `server.py` only) got the same mask.

## Checked against the old Python (completion pass)

Real handlers, the real bridge, `core_server.launch_kwargs`, against the old
`core_server.launch_options` + `gradio_overrides` + `print_gradio_options` (git
a33efc2), for six configs (plain, auth as a string, auth as a pair, ssl secrets,
legacy keys, auth + ssl + message) with no flags, `--share` and `--docker`: the
options are equal as Gradio sees them (the old single tuple is a one-element
list now); `--share` is the only addition (the old Go path ignored it), and
`--docker` is identical to plain. The printed lines are equal except
`ssl_keyfile_password` (`"sslsecretpassword..."` before, `"******"` now) and
the `share` value under `--share`. The launcher run (`TTS_GO_SERVERPY=1`, port
17900, no browser) reached the Gradio UI: 102 tabs listed through
`resolve`, 47 of them `[Disabled]` (the config disabled every non-builtin
extension to avoid importing torch), the built-in ones live.

## Stays in Python

`launch_kwargs` (auth pairs to tuples) and the call to `launch()` itself;
`main_block`, the torch-load patch and warning suppression. Without
`TTS_CORE_URL` the call raises `CoreBridgeError`.

## Differences from the old code

- `--share` works on the Go path.
- Several logins work; a malformed `auth` is refused instead of failing inside Gradio.
- `ssl_keyfile_password` is masked in the printed listing.
- The environment of the gradio child no longer has `TTS_GRADIO_PORT` and
  `TTS_GRADIO_HOST`.
- `core_server` no longer touches `os.environ` or `dotenv_manager.init`.

`core/cmd/serverpy-demo` is a prototype wiring that predates the config API (it
does not mount `/api/config` either) and was not updated.
