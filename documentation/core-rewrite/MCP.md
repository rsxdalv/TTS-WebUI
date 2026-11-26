# MCP server (spec)

Written 2026-11-26. Replaces the Python stub in PR #597 (a stdio JSON-RPC placeholder whose tools returned canned
text). **Status: draft, minimal first version.** Anything not run is marked *(not run)*.

## Goal

Let an MCP client (an agent, Claude Desktop, an IDE) turn text into speech with the engines this app already runs,
and nothing more. The server is a thin adapter: it holds no engine logic and no files of its own.

## Decisions

| # | Decision | Why |
|---|---|---|
| M1 | Written in **Go**, in the core module, as `core/internal/mcp`, started by `tts-webui mcp`. | The core is pure Go (D1) and already owns launch, supervision and the Host/Origin rules (D9). No Python process is needed to answer. |
| M2 | Built on the official SDK `github.com/modelcontextprotocol/go-sdk` (pinned, v1.8.0, Go 1.25+). | Version negotiation, notifications and error codes come right. The PR hand-rolled them and got several wrong. |
| M3 | **The backend is the OpenAI-compatible speech API** (`POST {base}/audio/speech`, `GET {base}/models`), default base `http://127.0.0.1:7778/v1` (the `openai_tts_api` extension). | It is the one stable, engine-neutral HTTP surface that exists today. The control API (token-guarded, process-starting) is deliberately **not** exposed to MCP. |
| M4 | **stdio is the default transport.** Streamable HTTP is opt-in with `--http ADDR`. | Desktop clients launch stdio servers, and stdio opens no port. |
| M5 | HTTP mode: loopback bind only, `Host` and `Origin` checked, `Authorization: Bearer` required. | A local MCP server is a DNS-rebinding and CSRF target; D9 applies. See Security. |
| M6 | **Tools only.** No resources, no prompts, no skills in v1. | They would be placeholders. Add one only when it carries real data. |

## Tools (v1)

| Tool | Input | Result |
|---|---|---|
| `list_models` | none | Text: the model ids the backend lists (`GET /models`). |
| `generate_speech` | `text` (required), `model`, `voice`, `speed` | One `audio` content block (WAV, base64, `audio/wav`) plus a one-line text note (bytes, duration if known). |

Rules for both:

- Inputs are plain values. **No file paths, no URLs.** The backend address is set by flag or environment only, never by
  a tool argument, so a model cannot make the server fetch an arbitrary address or read an arbitrary file.
- `text` is capped (default 5,000 characters). The answer is capped (default 20 MiB). Over the cap is an error result
  (`isError: true`), not a crash.
- Backend failures (unreachable, non-200, not a WAV) come back as `isError` results with a short message that never
  contains a key or header.
- The tool call honours the client's cancellation: the HTTP request to the backend is cancelled with its context.

## Configuration

| Flag | Environment | Default |
|---|---|---|
| `--backend URL` | `TTS_MCP_BACKEND` | `http://127.0.0.1:7778/v1` |
| `--http ADDR` | | off (stdio) |
| `--token-env NAME` | | `TTS_MCP_TOKEN` (HTTP mode only) |

The backend URL must be `http` or `https`; it is validated once at start. A bearer token for an upstream is out of
scope for v1 (the local extension has none).

## Security

- stdio: no listener, no auth needed; the parent process is the trust boundary.
- HTTP: refuses to start unless `ADDR` is a loopback address and a token is set (no token, no listener). The token is a
  secret the user chooses (at least 16 characters), compared in constant time. It is **separate from the per-run
  control token** and opens only `/mcp`.
- `Host` must be the bound loopback address or `localhost`; a request with a non-matching `Origin` header gets 403
  (the spec's Streamable HTTP rule). Requests without `Origin` (non-browser clients) pass this check and still need the
  token.
- No LAN mode in v1. Remote use goes through the user's own tunnel or reverse proxy, which is their decision.
- The server never logs request text or tokens.

## Non-goals (v1)

Transcription, chat, voice cloning and uploads, history/outputs resources, job queues and progress, OAuth, mounting on
port 3000 or `/mcp` on the React UI listener, Python-side tools. Each of these needs its own design; the first two
that are likely wanted are `transcribe` and an `outputs` resource.

## Tests

Unit tests with `httptest` standing in for the backend: tool listing, a successful `generate_speech`, an oversize
answer, a non-WAV answer, backend down, cancellation; HTTP mode: no token (401), wrong token (401), bad `Origin` (403),
bad `Host`, non-loopback bind refused. In-memory SDK transports are used for the protocol round trips.
**Not run against a real engine** until someone starts the extension on 7778 and records it.

## Open questions

1. Is the `openai_tts_api` extension's `/v1/models` and `response_format: wav` behaviour the same as the platform
   client assumes? (`core/internal/platform/openai` sends `wav`.) *(not run)*
2. Should a later version call the core's engine router directly instead of the OpenAI extension, so MCP works for
   engines that have no OpenAI wrapper? That needs a stable, token-free internal call path (D9) and is not decided.
3. Where does the HTTP token live for non-env use (settings file)? Deferred.
