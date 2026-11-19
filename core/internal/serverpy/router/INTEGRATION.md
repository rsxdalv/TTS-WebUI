# router: wiring notes (stage 3)

Package `core/internal/serverpy/router` replaces the Python proxy tree
(`tts_webui/gradio_proxy_tree`). Python side: `tts_webui/core_router_bridge/`
and `server_core.py` (repo root). Nothing existing was modified.

## What Rob must change in existing files

1. **Launcher, where the Python app is started** (`core/internal/launcher/supervisor.go`,
   the function that builds the `python server.py --no-react` command):
   - entry script `server.py` -> `server_core.py` (stage 1's supervisor takes it as config);
   - add these env vars to the child:
     `TTS_CORE_URL=http://127.0.0.1:<control port>`, `TTS_CORE_TOKEN=<token>`,
     `TTS_GRADIO_PORT=<private gradio port>`, `TTS_GRADIO_HOST=127.0.0.1`.
   - keep `--no-react` (Go runs the React UI itself).
2. **Launcher startup** (new code in the launcher, or wherever stage 1 starts things):

   ```go
   token := hex(32 random bytes from crypto/rand)          // new per launch
   rt := router.New(router.Options{Token: token, Logger: slog.Default()})
   rt.AddRoute("/", fmt.Sprintf("http://127.0.0.1:%d", gradioPort))
   mux.Handle("/api/routes", rt.ControlHandler())          // launcher's control mux
   go rt.Serve(ctx, router.BindHost(serverName), publicPort)
   ```
   - `publicPort` is the port users open (formerly 7770, the proxy tree's public port);
     `gradioPort` is a private free port (formerly 7767).
   - `serverName` is `gradio_interface_options.server_name` from `config.json`
     (`0.0.0.0` for `--docker`). `BindHost` maps "" to `127.0.0.1`, so the
     router is never more exposed than Gradio's configured server_name.
     Give Gradio itself `TTS_GRADIO_HOST=127.0.0.1` so only the router is public
     (the old design left Gradio on 0.0.0.0 in docker mode).
   - `--docker` handling moves to Go: the bridge removes `--docker` from argv, so
     Go must pass `serverName=0.0.0.0` and the ports itself.
   - Mount `ControlHandler` on a listener that is **loopback only**; it also
     refuses non-loopback peers and wrong tokens (403) and non-JSON POSTs (415).
     Do not mount it on the public router port (the router would send
     `/api/routes` to Gradio anyway).
3. **Stage 4 (extprocs)**: pass the same `*router.Router` as its `RouteRegistrar`.

Opening the browser (`inbrowser=False` is forced in Python) is Go's job.

## Behaviour notes / deliberate differences from the Python proxy

- Matching, prefix stripping, Host / X-Forwarded-Host / X-Forwarded-Proto, hop-by-hop
  removal, JSON 404 (`{"detail":"Not Found (proxy server - no route matches /x)","proxy":true}`),
  502 `Proxy error: upstream unavailable`, 500 `Proxy error: internal error`,
  `GET /healthz` and `GET /routes` while nothing routes "/", all as in Python.
- No X-Forwarded-For is added (Python did not); a client-supplied one passes through.
- Go passes request `Content-Encoding`/`Content-Length` and response bodies byte
  for byte (Python dropped them because httpx decoded). WebSocket handshakes keep
  all client headers (cookies, Origin, subprotocols); Python only forwarded Host.
- `X-Forwarded-Proto` is `http`/`https` for WebSockets too (Python sent `ws`).
- The control API refuses the `/` route: Go owns the catch-all via `AddRoute("/", ...)`.
  Targets from the control API are always `http://127.0.0.1:<port>`.
- `setup_proxy_extension.add_route_once` is `lru_cache`d, so if the bridge's POST
  fails (after 3 attempts) it is not retried until restart; stage 4 registering
  the route from Go when it starts the extension avoids that.
- Control API: POST/DELETE return 200 with a small JSON body; unknown path 404;
  other methods 405; body > 16 KiB 413; an empty configured token refuses everything.

## Verified

Really (Go 1.27.4, Python from `installer_files/env`, ports 55055-55057):
- a throwaway main ran the router on a free port with `/` -> Gradio and the control API
  on another; `server_core.py --no-react` with `TTS_CORE_*` started Gradio 5.49.1 on the
  private port (Python proxy tree not started, no browser);
- `GET /` (1.69 MB HTML), `/config`, `/gradio_api/info` through the router: 200;
- a Gradio queue run (`POST /gradio_api/queue/join`, then the SSE stream
  `/gradio_api/queue/data`, chunked `text/event-stream`) delivered estimation,
  process_starts, process_completed through the router;
- the bridge's `register_route` against the real control API added `/demoext`
  (a request to it gave 502 as nothing listened); `DELETE` removed it; no token gave 403.
Only in tests (httptest/fakes): WebSocket echo (hand-written handshake and frames),
incremental SSE, header handling, longest prefix, escaped paths, control auth,
graceful shutdown; Python bridge against a stub server with fake `main`/`config` modules.
Not verified: a real proxy extension behind the router, `--docker`, TLS, Windows
firewall prompts on non-loopback binds.

Run tests:

    cd core && go test ./internal/serverpy/router/...
    PYTHONPATH=. python -m unittest tts_webui.core_router_bridge.test_core_router_bridge
