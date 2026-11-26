# MiniMax Cloud TTS in Go (`cloud/ext-minimax-go`)

Draft. The builtin extension `extension_minimax_cloud_tts` is a Gradio tab that makes one HTTPS call to MiniMax's text-to-speech
API. Its Vite page (`/text-to-speech/minimax-tts`) called the Gradio endpoint `minimax_cloud_tts`, so it needed the Gradio host.
This branch makes the same call in Go and points the Vite page at it. The Gradio tab and the Python extension are not touched
(`git diff main -- extensions tests` is empty).

**The real MiniMax service was never called: there was no key.** Everything below that touches it is read from the Python
extension and from MiniMax's public reference, and tested against a stand-in server. See
[Not verified: real MiniMax service (no key)](#not-verified-real-minimax-service-no-key).

Earlier decision: `LEAF-TABS.md` item 10 recorded "MiniMax is not part of the core" and dropped an earlier Go handler. This
branch is a new, explicit request, kept outside the platform packages (`internal/cloud/minimax`), and is only a draft until Rob
decides.

## What changed

| Where | What |
|---|---|
| `core/internal/cloud/minimax/` (new) | `store.go` settings and key; `client.go` the call, `Speak`, `Test`; `handler.go` control API `/api/minimax/`; `minimax_test.go` |
| `core/internal/platform/openai/shared.go` (new), `clip.go`, `save.go`, `client.go` | The P5 helpers MiniMax shares are exported instead of copied: `Scrub`, `Snippet`, `ShortErr`, `Unreachable`, `CheckTransport`, `WriteFileAtomic` (0600), `ReadJSON`, `Invalidf`, `Authorized` (loopback + token), `WriteJSON/WriteErr/DecodeJSON`, `NoRedirectClient` (the redirect rule, moved out of `NewClient`), `CheckWav`. `SaveClipFields` is `SaveClip` for a generator with its own metadata fields (`meta` got `Version` and `Fields`; the default output of the openai platform is byte for byte what it was, its tests are unchanged). |
| `core/internal/launcher/serverpy.go` | mounts `/api/minimax/` on the control API next to `/api/openai/` |
| `core/react_ui_backend/core_minimax.go` (new) | browser routes, switch `core-minimax` (below) |
| `react-ui/src/functions/generateWithMiniMaxTTS.ts`, `tabs/MiniMaxTTSParams.ts`, `components/MiniMaxTTSInputs.tsx`, new `components/MiniMaxKeyCard.tsx`, `routes.tsx`, `components/Header.tsx` | the page uses the Go routes; the API Key field became a settings card; the page is also at `/minimax` with a menu entry |
| `core/scripts/smoke/minimax-no-key-leak.sh` (new) | the real-binary version of the no-key-leak test |

`core/ui` (the launcher UI behind `core/internal/webui/assets`) is untouched. I ran `core/scripts/build-webui.sh`: it reproduces
the committed assets byte for byte (`git status` clean, `bundle.hash` unchanged), so there is nothing to commit there and the CI
check "Build core UI and check committed assets" compares equal hashes.

## Routes

Browser, port 3000, under `/api/go-data/minimax/` (switch `core-minimax`; the Go side adds the core token):

| Route | Who | Does |
|---|---|---|
| `GET settings` | anyone who reaches the UI | `{base_url, model, voice, has_key, key_source}`; `key_source` is `stored`, `env` or `none`. Never the key |
| `PUT settings` | this machine only | `{base_url, model, voice, key?}`; `key` absent keeps the stored key, `""` removes it; an empty `base_url`, `model` or `voice` goes back to the default |
| `POST test` | this machine only | one speech request for "Test."; `{ok, message}`; saves nothing; uses a few characters of quota |
| `POST speech` | anyone who reaches the UI | `{text, model?, voice_id?}` (no key field: unknown fields are a 400) -> `{folder, wav, png, json, seconds, bytes, text, model, voice_id}` |

"This machine" is the P5 rule (`thisMachine`): a loopback peer and none of `X-Forwarded-For`, `Forwarded`, `X-Real-Ip`,
`X-Forwarded-Host`. Errors: 400 input (including "no MiniMax API key"), 403 not this machine, 502 MiniMax's side or the way
to it (the text is safe to show), 500 ours.

## Key handling (same rules as P5)

- The key is stored by Go in `platforms/minimax/key.json` (mode 0600, written atomically, `/platforms/` is in `.gitignore`);
  the settings are in `settings.json` beside it. It is read from `MINIMAX_API_KEY` when none is stored (the Python extension does
  the same); a stored key wins; an environment key is never written to disk.
- No answer carries it, an error text has it removed (`Scrub`; MiniMax's `status_msg` is shown, scrubbed), the log lines are
  the outcome and the folder only. The test `TestLogsNeverCarryTheKey` feeds a key through an error and checks the log.
- A key is not sent in clear text over the internet: `PUT` is refused for a plain `http://` host unless it is this machine or a
  private address (the environment key counts). Redirects are never followed (a key would travel to a host the user did not
  enter); they are reported.
- The page no longer keeps the key anywhere: the old field stored it in `localStorage` (`minimaxTTSParams.v1.api_key`) and
  sent it with every Gradio call. The new field is cleared after Save and never filled from the server; a copy left in
  `localStorage` by an older page is dropped when the page opens.

## The API as implemented, and where each part comes from

| Part | Value | Source |
|---|---|---|
| URL | `POST <base_url>/v1/t2a_v2`, default `base_url` `https://api.minimax.io` | the extension (`MINIMAX_API_URL`); MiniMax reference |
| Auth | `Authorization: Bearer <key>`, `Content-Type: application/json` | the extension; MiniMax reference (`bearerAuth`) |
| Request | `{"model", "text", "voice_setting": {"voice_id"}, "audio_setting": {"format": "wav"}}` | the extension, except `format` (below) |
| Defaults | model `speech-2.8-hd` (also `speech-2.8-turbo`), voice `English_Graceful_Lady`, the same 12 voices in the page | the extension and the old page |
| Response | JSON; `data.audio` is the audio as hex; `base_resp.status_code != 0` is an error with `status_msg` | the extension (it reads exactly these) |
| Codes worded | 1004 key refused, 1002 and 1039 rate limit, 1001 timeout, 1042 invalid characters over 10%, 2013 invalid parameters; any other code is shown as `MiniMax error <code>: <msg>` | MiniMax reference (the list on that page; insufficient balance is not on it, so it falls under "any other") |
| Text limit | under 10,000 characters, checked before the call | MiniMax reference |
| Not sent | `stream` (default false), `output_format` (default `hex`), `speed`, `vol`, `pitch`, `sample_rate`... | the extension does not send them either |

"MiniMax reference" is https://platform.minimax.io/docs/api-reference/speech-t2a-http, fetched on 2026-11-26 by a helper
agent that read the OpenAPI text of the page and reported it; I did not read the raw page myself. The helper also reported
that it did not fetch the error-code reference page (`/api-reference/errorcode`) or the "US West" endpoint page.

**Where this branch departs from the extension: `format`.** The extension asks for `mp3`. The History page lists a clip by its
`.png` and `.wav` (see below), and Go has no MP3 decoder (no dependency was added), so Go asks for `wav`. The reference lists
`wav` among the `audio_setting.format` values and does not restrict it to non-streaming for that field. **Open point:** the same
page's `extra_info.audio_format` enum is `[mp3, pcm, flac]`, without `wav`, so the reference contradicts itself. Whether the real
service honours `wav` is not known. If it answers something that is not a WAV, the call fails with a message that says so
(`MiniMax sent audio that is not a WAV file (it looks like MP3) although wav was requested`) and nothing is saved.

Unknown, not guessed: the regional hosts (the reference mentions a US West endpoint without saying its host here; the `API
host` field exists for that), the insufficient-balance code, rate limits per plan, whether the key is valid for both
`speech-2.8-*` models, and the shape of an HTTP-level (non-200) error body from MiniMax (the Go side shows a snippet of it).

## The saved layout

Finding while checking against the History reader: **the Python extension's own folder is not listed by History.** It writes
`outputs/<YYYYMMDD_HHMMSS>_<text>/<text>.mp3` and `.json`. History (`serverpy/history`: `Items`, `Rows`) lists a folder by a
`.png` with a sibling `.wav`, its date comes from a `YYYY-MM-DD_HH-MM-SS` stamp in the folder name, and the Vite Outputs page
(`data_outputs.go`) needs a truthy `date` in the JSON and plays only `.ogg` or `.wav`. The Python folder has none of these
(`TestLayoutMatchesALegacyGenerationAndPythonsFolderIsNotListed` builds one and shows `Items` skipping it).

So Go writes the layout of a real legacy generation, the one P5 checked against the Chatterbox fixture
`platform/openai/testdata/legacy/outputs/2026-04-07_20-07-40__chatterbox__hi_ma_na_is/`:

```
outputs/2026-11-26_01-07-14__minimax__Hello_from_the_/2026-11-26_01-07-14__minimax__Hello_from_the_.{wav,json,png}
```

built under `outputs/.incoming-*` and renamed, a second clip in the same second gets `_2`. The JSON keeps the Python
extension's fields and adds the legacy ones:

```json
{ "_version": "0.1.0", "_hash_version": "0.0.2", "text": "...", "model": "speech-2.8-hd", "voice_id": "English_Graceful_Lady",
  "provider": "minimax", "_type": "minimax_cloud_tts", "response_format": "wav",
  "outputs": null, "date": "2026-11-26 01:07:14.443070", "hash": "<sha256 of the samples>" }
```

Differences from the Python extension's folder: `.wav` (what MiniMax sent) instead of `.mp3`, a Go-drawn waveform `.png`, folder
name `<date>__minimax__<title>`, `response_format`, `_hash_version`, `outputs`, `date`, `hash`. Existing Python-written MiniMax
folders stay as they are (not listed by History, as before).

## Evidence

All on Linux (Ubuntu runner, CPU only, no GPU, no key). Go 1.27.0 from `core/go.mod`.

1. **Tests with a stand-in MiniMax** (`httptest`, `internal/cloud/minimax/minimax_test.go`, run with `-race`): the documented
   request is what the stand-in receives (method, path, bearer, body fields); success saves `.wav/.json/.png` and the wav equals
   what was sent; wrong key (HTTP 200 + `base_resp` 1004 that echoes the key; the error and the log do not carry it); HTTP 401,
   403, 404, 429, 500, 502 with a `base_resp`, 400; every `base_resp` code above and an unknown one; malformed answers (not JSON,
   truncated JSON, `{}`, empty audio, null data, audio that is a number, a URL, not hex, odd hex, MP3 bytes, unknown bytes, a WAV
   with no samples, a truncated WAV header) each refused and nothing saved; an answer over the size cap (cap 128 MiB, 1 MiB in the
   test); a redirect (reported, the target is never contacted, the key does not follow); closed port; cancelled request; input
   rules (empty, 10,000 characters, multi-line model, characters counted as characters); no key; `MINIMAX_API_KEY` and stored
   key priority; store rules (defaults, transport rule, `/v1` suffix refused, file mode 0600, only `key.json` holds the key).
2. **Layout against a real legacy generation**: the same `history` package code (`Items`, `Rows`, `Select`) over the Chatterbox
   fixture, a Go-written clip and a Python-shaped MiniMax folder: same row shape and `select` keys, same file extensions,
   metadata has every legacy field.
3. **No route returns the key** (`react_ui_backend/core_minimax_test.go`, through the real front door, claim, control API and
   store): every route and error path, then every answer body and every file under the install root are searched for the key;
   only `key.json` holds it. The same test checks the LAN rules (PUT and test from a non-loopback peer, a forwarded request and a
   cross-site Origin are refused; read and speech stay open), that the clip is listed by `/api/go-data/outputs/outputs` and that
   its player URL answers `audio/wav`.
4. **The real binary**, `go build ./cmd/tts-webui`, scratch root with `react-ui/out`, `tts-webui --no-browser --start`, no Python
   and no Gradio host (the log says "the Gradio host is not installed"), a small Python stand-in for MiniMax on 127.0.0.1:
   settings, test, generate by `curl`; `platforms/minimax/key.json` mode 600; the clip listed by `/api/go-data/outputs/outputs`;
   `GET /api/webui-generations/outputs/<folder>/<folder>.wav` answers `200 audio/wav` (88244 bytes); the launcher log has no key.
5. **The page in a browser** (headless Chromium, the page at `/minimax`, no Gradio host): a wrong key then Test shows "MiniMax
   refused the key (1004, authentication failed): check the API key: login fail: [key] (trace id mock)" (the stand-in echoed the key,
   Go removed it); the key field is empty after Save; the right key then Test shows "MiniMax answered a speech request with
   2.0 s of audio."; Generate plays the clip with its waveform from `/api/webui-generations/...`; the browser's `localStorage`
   and the page HTML hold no key; the only calls are `/api/go-data/minimax/*` and the clip URL (no `/api/gradio/`).
6. **`core/scripts/smoke/minimax-no-key-leak.sh http://127.0.0.1:3000 <root>`** against that binary printed
   `PASS: no answer and no file holds the key`; with a file planted that holds the key it prints `FAIL: key left in: ...`.
7. **Per commit**: `gofmt -l .` (empty), `go vet ./...`, `go test ./...` in `core`, all green. `react-ui`: `node
   generate-plugin-imports.js`, `npx tsc --noEmit` (no errors) and `npm run build:static` (builds) as in `test-react-ui.yaml`.

## Not run

- Windows and macOS (out of scope for this branch). Nothing here is Linux-only, but nothing was run elsewhere.
- The UI open to the LAN with a second device (`TTS_GO_PROXY_LISTEN=lan`): the rule is tested at the route level with fake peer
  addresses, as in P5, not with a real second device.
- The "Favorite" button on the page still calls the Gradio endpoint `bark_favorite` (shared `favorite()` helper, unchanged), so it
  needs the Gradio host. Generate, play, Use Parameters and the History pages do not.
- Streaming output, `speed`/`vol`/`pitch`, `output_format: url` and subtitles are not implemented (the Python extension has none).
- A fresh `npm ci` in `core/ui` was run by `build-webui.sh`; the CI job itself was not run here.

## Not verified: real MiniMax service (no key)

**Unverified:** every call to the real `api.minimax.io`, in particular:

- that `audio_setting.format: "wav"` is honoured (the reference lists it and also contradicts itself, see above); if it is
  not, no clip can be made until the format question is settled (the extension's `mp3` works for the Gradio tab, but Go cannot
  save it as a History clip without a decoder);
- the key sent as `Authorization: Bearer <key>` and accepted (the stand-in only compared the string);
- the exact answer for a wrong key (the stand-in uses HTTP 200 with `base_resp` 1004 as the extension's code path and the
  reference suggest) and for rate limits, quota and a bad voice id;
- that `data.audio` is hex of a complete WAV file with a header, not raw PCM;
- TLS to a real host; the speed and size of a long text (the 128 MiB cap is a guess: 64 MiB of audio is about 17 minutes of
  16-bit mono at 32 kHz).

**Verified instead:** everything on the Go side of that call against a stand-in that follows the Python extension's code path
and the reference (Evidence 1 to 6).

**To close it** (Rob types the key; nobody else stores one). In a scratch folder or an install:

1. Optional, outside the app, the call alone (needs `curl` and `python3`; the key is read from the environment, never typed in
   the command):

   ```bash
   export MINIMAX_API_KEY='<your key>'
   curl -s https://api.minimax.io/v1/t2a_v2 \
     -H "Authorization: Bearer $MINIMAX_API_KEY" -H 'Content-Type: application/json' \
     -d '{"model":"speech-2.8-hd","text":"Test.","voice_setting":{"voice_id":"English_Graceful_Lady"},"audio_setting":{"format":"wav"}}' \
     | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d["base_resp"]); print(bytes.fromhex(d["data"]["audio"])[:12])'
   ```

   Expect `{'status_code': 0, 'status_msg': 'success'}` and `b'RIFF....WAVE'`. If it prints an MP3 header (`b'ID3...'` or
   `b'\xff\xfb...'`), or a non-zero `status_code`, the `wav` assumption is wrong or the key is not valid: stop and write down the
   output (without the key).
2. Run the app: `tts-webui --no-browser --start` (or the launcher), open `http://127.0.0.1:3000/minimax`. The Gradio host does not
   need to be installed.
3. Type the key in API Configuration, Save, then Test. Expect "MiniMax answered a speech request with N s of audio."
   If the account's host is not `https://api.minimax.io`, put the right host in "API host" first (host only, no `/v1`).
4. Generate with some text, both models and a couple of voices. The clip must play on the page, appear at
   `/outputs/outputs` (History), and the `.json` next to it must carry `provider`, `_type`, `model`, `voice_id` and `date`.
5. Type a deliberately wrong key, Save, Test: expect "MiniMax refused the key (1004, authentication failed)" or the "HTTP 401"
   wording; note which one the service really gives.
6. In the Gradio tab (Gradio host installed) generate once with the same key and voice: the same key must work there; compare the
   clip's length with the Go one.
7. Run `core/scripts/smoke/minimax-no-key-leak.sh http://127.0.0.1:3000 <install root>`: it must print PASS (run it with no key
   stored: it stores a sentinel and removes the key at the end, so save yours again afterwards).
8. Optional: unset the stored key (Remove key), set `MINIMAX_API_KEY` before starting the app: the card must say "Using the
   MINIMAX_API_KEY environment variable." and Test must pass.

Then replace this section's "Unverified" list with what was seen, and note the date.
