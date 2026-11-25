# platform/openai: the `openai` platform (P5)

Plan: [PLAN.md](../../../../documentation/core-rewrite/PLAN.md) item P5, decision D27. An endpoint is a URL (and a key) of a
server that answers `POST <URL>/audio/speech`. Nothing is installed and no process runs; Go makes the call and saves the
clip. No Python.

## What it is

| Part | Where |
|---|---|
| Endpoints and keys | `platforms/openai/endpoints.json` (name, URL, model, voice) and `platforms/openai/keys.json` (name to key), mode 0600, written atomically. `/platforms/` is in `.gitignore`. `store.go` |
| The call | `client.go`: `Speak` (always asks for `response_format: wav`; the answer must be a WAV), `Test` |
| The saved clip | `save.go`, `wav.go`: `outputs/<date>__<model>__<title>/<same>.{wav,json,png}` |
| Control API (loopback, `X-TTS-Core-Token`) | `handler.go`, mounted by `launcher/serverpy.go` at `/api/openai/` |
| Browser routes on port 3000 | `react_ui_backend/core_openai.go`, switch `core-openai`: `GET/PUT/DELETE /api/go-data/openai/endpoints[/{name}]`, `POST .../{name}/test`, `POST /api/go-data/openai/speech` |
| The page | `react-ui/src/pages/openai.tsx`, route `/openai`, top-level menu entry (a `/text-to-speech/...` path is a Gradio path and is hidden without the Gradio host) |
| Seam | `platform.go`: a `platform.Platform` that is always ready; `Install` says it is not installable; `Command` answers that an endpoint has no process |

## Rules and how each is kept

- **The key is never returned.** It enters only by `PUT` (`key`; absent keeps the stored one, empty removes it). `List`, `Put` and
  every error answer carry `has_key` only. Error text from the remote has the key removed (`scrub`), and the log lines hold
  the endpoint name and the outcome. Tests `TestOpenAIRoutesNeverReturnTheKey` (every route and error path, every file
  under the root) and `core/scripts/smoke/p5-no-key-leak.sh` (the real binary).
- **Settings change from this machine only.** `PUT`, `DELETE` and the test call answer 403 unless the peer address is loopback and
  no `X-Forwarded-For`, `Forwarded`, `X-Real-Ip` or `X-Forwarded-Host` header is present. Listing and generating stay open
  to a LAN browser. Run for real with `TTS_GO_PROXY_LISTEN=lan` (below).
- **A key is not sent in clear text over the internet.** Plain `http://` with a key is accepted for loopback, `localhost` and private
  or link-local addresses; anything else needs `https://`. Redirects are never followed (a key would travel to a host the
  user did not enter); they are reported.
- **The saved folder is complete or absent:** it is built as `outputs/.incoming-*` (History skips names starting with `.`) and
  renamed. A second clip in the same second gets `_2`.

## Layout checked against a real legacy generation

`testdata/legacy/outputs/2026-04-07_20-07-40__chatterbox__hi_ma_na_is/` is a folder a real Chatterbox generation wrote (the
`.flac` and `.ogg` an older version also left there were removed from the fixture). `TestLayoutMatchesALegacyGeneration` puts it
and a Go-written folder in one root and runs the History package (`Items`, `Rows`, `Select`) over both: same list shape, same
row shape, same `select` keys, same file extensions, and the metadata has `_version`, `_hash_version`, `text`, `outputs`, `date`,
`hash` with the same date and hash shapes. File and folder names follow `decorator_add_base_filename.py` including
`prompt_to_title` (`TestPromptToTitleMatchesPython`).

Differences from a legacy clip: the `.wav` is whatever PCM or float WAV the endpoint sent (legacy: float32 from numpy); the `.png`
is a 1000 x 300 waveform drawn by Go (legacy: a plot); `hash` is the SHA-256 of the WAV's sample bytes (legacy: of the numpy array
bytes, the same bytes for a mono clip of the same sample type); the metadata also has `endpoint`, `model`, `voice`, `speed`,
`response_format`.

**One fix outside this package:** `react_ui_backend/data_outputs.go` named `<dir>.ogg` as the audio of every folder, so the Vite
History page's player was a 404 for any generation without an `.ogg`, legacy Chatterbox ones included. A folder with a `.wav` and no
`.ogg` now points at the `.wav`; the rest is as before (`TestDataOutputsPointsAtTheWavWhenThereIsNoOgg`).

## Verification (2026-11-25)

Windows 10, Go 1.27.4, scratch copy of `git archive HEAD` under `%TEMP%\p5fresh` with no Python, git or Node on the launcher's path
needs; Linux: WSL `tts-linux-test`, cross-built binaries, root copied to `/tmp/p5lin_root`. The remote was a **stand-in**: a 30-line
Go server (`/v1/audio/speech`, checks `Authorization`, returns a 2 s tone as WAV). **No real remote endpoint and no real key were
used.**

| Line | Windows | Linux (WSL) |
|---|---|---|
| `gofmt`, `go vet ./...`, `go test ./...` in `core` | pass | the test binaries of `platform/openai` and `react_ui_backend` pass |
| The `/openai` page in the real binary (browser pane, driven by DOM events: the pane was hidden): add an endpoint with a wrong key, Test, Generate, edit with the right key, Test, Generate | wrong key: "the endpoint refused the key (HTTP 401): check the key of the endpoint"; right key: 2.0 s speech test and a clip; the page never shows the key (`Edit` leaves the field empty) | same calls by curl: PUT, test, wrong-key 401 error, generate, clip `RIFF ... 16 bit, mono 22050 Hz`, `keys.json` mode 600 |
| The clip in History: `/api/go-data/outputs/outputs` and the Vite page `/outputs/outputs` list it, the player URL answers 200 `audio/wav` (88244 bytes) | yes | list and wav 200 by curl |
| Wrong URL (404 path, unknown host, https to an http port, closed port) | each answers 502 with a sentence naming the URL or host | not run |
| `p5-no-key-leak.sh` | PASS | PASS |
| UI open to the LAN (`TTS_GO_PROXY_LISTEN=lan`), requests to this machine's LAN address | `PUT`, `DELETE`, test: 403; list and generate: 200; test from 127.0.0.1: 200 | not run |
| Endpoint list survives a launcher restart | yes | not run |

### Not run

- **A real remote endpoint** (acceptance line 1): no key was given.
- **The local `openai_tts_api` extension on port 7778 with a local engine** (line 4; needs the Gradio host and an engine).
- A second device (the LAN rule was exercised with this machine's own LAN address, whose peer address is not loopback), macOS, a
  remote that streams the WAV with chunked transfer (parsing of a streamed header is unit-tested), an MP3-only endpoint (it is
  refused with "not a WAV file"), clips over 256 MB.
