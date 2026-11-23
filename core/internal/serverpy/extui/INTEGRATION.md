# extui: the Extensions List pages, served by Go into Gradio

The two list tabs (Extensions List, Decorator Extensions List) are an iframe of
`/_core/ext/list?kind=interface|decorator` (`tts_webui/extensions_loader/list_tab.py`).
Go serves the page and its API from the same origin as Gradio: the router has a
route `/_core` (prefix stripped) to a loopback listener of its own, started in
`launcher/serverpy.go` (`startExtUI`). Python has no handler for these tabs and
asks Go nothing for them.

The Gradio API endpoints `install_extension` and `uninstall_extension` that the
old tab registered are gone with its handlers (nothing in the repo called them).

## Paths (after the router strips `/_core`)

| Path | What |
|---|---|
| `GET /ext/list?kind=` | the page; sets the cookie and embeds the token |
| `GET /ext/ui.js`, `/ext/ui.css` | its assets (embedded, `assets/`) |
| `GET /ext/api/list?kind=` | entries (title, description, website, `installed`, `version`, `disabled`, `builtin`), the install picker's `choices`, the `uninstallable` subset, `read_only`, a `warning` when pip could not be read |
| `POST /ext/api/op/{install,uninstall}` | `{"package_name"}`; passed to extcatalog's own handler, NDJSON back |

## Why this is not a new hole

The control API (loopback + `X-TTS-Core-Token`, held by Python) is not on this
route and not on this listener. The browser gets a second, narrower surface:

1. **Host**: a loopback name and the public port, else 400 (DNS rebinding).
2. **Cookie** `tts-ui-<port>`: random per start, `HttpOnly`, `SameSite=Strict`,
   `Path=/_core/` (Gradio and the extensions never receive it). Needed by every
   `/ext/api/*` call.
3. **A POST also needs** the page's token in `X-TTS-UI` (a second random value,
   not the cookie), an `Origin` that equals the Host (absent or `null` is
   refused), `Content-Type: application/json`, and `Sec-Fetch-Site` not
   cross-site.
4. **Allow-list**: install and uninstall by package name, which extcatalog looks
   up in the lists and never takes requirements from the caller. `install-entry`,
   `parse`, `add`, `check-dependencies`, `latest-version` are not reachable.
5. **Framing and scripts**: CSP `default-src 'none'; script-src 'self';
   style-src 'self'; connect-src 'self'; frame-ancestors 'self'` plus
   `X-Frame-Options: SAMEORIGIN`. Extension text is only set with `textContent`;
   links only for `http(s)` URLs, with `rel=noopener noreferrer`.
6. **Open to other devices** (the router listens beyond loopback: `--listen`,
   Docker): any Host passes and the page is **read-only**; every POST gets 403.
   There is still no password, so installs stay off there until there is real auth.

Not covered: another process of the same user can start its own request with
`curl`, set its own Origin, and read the cookie only if the browser sends it
to that process (the cookie's path keeps it off every other route of this origin,
but cookies ignore ports, so a different local web server on `127.0.0.1` can see
it). It would still need the token, which is only in the page. That is the same
level as any local process running `pip install` itself.

## Checked

- `go test ./internal/serverpy/extui`: every rule above (each guard was broken on
  purpose to see its test fail), plus `launcher.TestServerPyServesExtensionPagesThroughTheRouter`
  (page, cookie, API through the public port; the control API is a 404 under `/_core`).
- In a real browser against the handler mounted at `/_core`: the table renders, an
  install streams into the console, an iframe from the same origin loads and
  works, and the same page embedded from `localhost` (another origin) is blank.
- **Not run:** inside the real Gradio page with a full install of the app, and
  an install that changes the main env (the tests use a command that fails).
