# Docker image check (P4 acceptance line 4: "the Docker image builds and starts")

Run on 2026-11-26 on an Ubuntu cloud runner (Docker Engine 29.9.3, buildx 0.39, compose 5.7, 4 CPUs, 15 GB RAM, **no GPU,
no NVIDIA runtime**), from `main` at b844e0a. The P4 author's machine had no Docker daemon, so the line had never been run.
Scratch folders were outside the checkout; the repository was not used as a working directory for any volume.

**Result in one line:** the image builds, but as merged it did **not start the app** (Gradio never launched, `python` was
missing) and the React UI port the compose file publishes was unreachable. Both are fixed in this branch and re-checked
on a rebuilt image. An engine installed and generated a clip on the uv platform; History could not be verified.

## What this branch changes

| Commit | Defect | Fix |
|---|---|---|
| `Dockerfile` | The launcher runs `--system-python`, and the `system` platform starts `python` from PATH (`platform/system/system.go`). The CUDA base is Ubuntu 22.04, which has `python3` only. Log: `WARN gradio not started: no Python environment: python was not found on PATH`; `:7770` answered `502 Proxy error: upstream unavailable`. | `python-is-python3` added to the apt line. |
| `docker-compose.yml` | The compose file publishes `3000`, but the Go proxy on 3000 listens on loopback inside the container (D17, D20), so `curl :3000` got `Connection reset by peer`. | `TTS_GO_PROXY_LISTEN=lan` and `TTS_GO_PROXY_ALLOWED_HOSTS=${TTS_ALLOWED_HOSTS:-localhost,127.0.0.1}`. Other `Host` names still get 400 (checked). Whoever opens the UI by a name or LAN address must add it to `TTS_ALLOWED_HOSTS`. This is a posture choice (the launcher logs "reachable from the network ... with no password" at every start); the compose file already published 7770 with no password. The image itself still defaults to loopback for 3000. |

No Go code changed, so `gofmt`/`go vet`/`go test` were not re-run for this branch.

## Runner workarounds (not changes to the repo)

The runner's network is a TLS-intercepting egress proxy, which containers do not trust. To build the **unchanged**
Dockerfile:

- Docker Hub answered `429` to the daemon (`golang`, `node`, `nvidia/cuda`); `mirror.gcr.io` was set as the daemon's
  `registry-mirrors` (`/etc/docker/daemon.json`).
- The three base images were re-tagged locally (`golang:1.27`, `node:22`, `nvidia/cuda:12.8.0-devel-ubuntu22.04`) with
  the proxy CA appended to `/etc/ssl/certs/ca-certificates.crt` and `SSL_CERT_FILE`/`PIP_CERT`/`NODE_EXTRA_CA_CERTS` set.
  Because the final image derives from that tag, **the checked image carries the proxy CA and those variables**. The
  published image does not.
- `docker build --network host --build-arg HTTPS_PROXY=... --build-arg NO_PROXY=...` (the proxy listens on 127.0.0.1).
- Containers cannot reach that proxy from a bridge network. Port publishing and shutdown were checked with `docker
  compose` on the bridge network (no egress); the engine install used `docker run --network host` with the proxy.
- The first rebuild ran out of the runner's disk allowance (`pip: No space left on device`, not an image defect); old
  images and the build cache were removed and the build repeated.

## Commands and results

### 1. Build

```
docker build --network host --progress=plain --build-arg HTTPS_PROXY=$HTTPS_PROXY ... -t tts-webui:check .
```

| | |
|---|---|
| Time | **10 min 27 s** cold (parallel stages: Go 22 s, Node 23 s; then apt 37 s, torch/xformers cu128 120 s, `requirements.txt` 84 s, rvc 53 s, styletts2 22 s, export and unpack of the layers 306 s). Rebuilt after the Dockerfile fix: 11 min 31 s (cold; the apt layer changed) |
| Size | **10.5 GB** (content size; the layers are the CUDA devel base 5.2 GB plus torch and the requirements) |
| Result | success, no build error |
| Warnings | `npm warn ERESOLVE overriding peer dependency` (react-remove-scroll wants `@types/react` 16-18, the app has 19); apt `update-alternatives: warning: skip creation of /usr/share/man/...` (vim/nano without man pages); `pip: Running pip as the 'root' user ...`. No pip resolver conflict was printed |

Image contents (`docker run --entrypoint sh`): `uv 0.13.6` at `/usr/local/bin/uv`, `Python 3.10.12`, **no `node`, no `npm`,
no `micromamba`**, `react-ui/out` present (`_tts-export.json`, `assets/`, `index.html`, `favicon.ico`). `Config.User` is
empty (root), no `ExposedPorts`, entrypoint is the NVIDIA one, `Cmd` is
`tts-webui launch --root /app/tts-webui --docker --start --system-python --no-browser`.

### 2. Start (compose, scratch folder, `image:` swapped to the local tag, GPU reservation removed)

The repo's `docker-compose.yml` was copied to a scratch folder and run with an override file that sets only `image:` and
resets `deploy:` (no NVIDIA runtime here).

As merged (image `:check`): launcher up on `127.0.0.1:7700` in the container, router on `0.0.0.0:7770`, then
`WARN gradio not started: no Python environment: python was not found on PATH`, `:7770` 502. With a `python` symlink
layered on top, Gradio loaded 69 entries and `:7770` answered 200, `:3000` and `:7778` reset. This is what led to the two
fixes above.

With the fix (image `:fixed`, the repo's compose file as in this branch):

| Check | Result |
|---|---|
| `curl :7770/` (Gradio through the router) | 200 |
| `curl :3000/`, `localhost:3000/`, `:3000/api/go-data/host-status` | 200, 200, 200 |
| `curl -H 'Host: evil.example:3000' :3000/` | 400 |
| `curl :7778/` | connection reset: nothing listens until the `openai_tts_api` extension is activated (by design, the extension binds 0.0.0.0:7778 on "Activate API") |
| Launcher page and API | the launcher (7700) is **not published** by compose; reached with `docker exec ... python3` (page carries `<meta name="tts-webui-token">`, API takes `X-TTS-Token`). `/api/app/status`: gradio ready on 7770, react-ui ready on 3000 |
| `system` platform detected | `GET /api/ext/list`: `platform: system`, `platform_ready: true`, 69 entries |
| `requires` on the Extensions list | 27 `gradio-host` entries `ready via system`; 40 `uv` entries `missing`; 1 `onnx` missing; 1 `audiocpp` missing. Every entry carries `requires` and `requirements_state` |
| uv on PATH, no micromamba | `which uv` = `/usr/local/bin/uv`, `uv 0.13.6`; `which micromamba` empty; `which python` = `/usr/bin/python` (after the fix) |
| Shutdown | `docker compose stop`: 1.5 s, exit code 0, log `launcher exiting: stopping the app`, Gradio `Keyboard interruption ... closing server`. PID 1 is the launcher (`tts-webui`), Gradio is its child |

Notes: the launcher logs `WARN ... React UI is reachable from the network at http://172.18.0.2:3000, with no password`
at every start now (see the first table). A fresh container logs `serverpy: .env not found, created the default`.

### 3. Install one CPU engine on the uv platform and generate a clip

Container started with `docker run --network host` (proxy env, scratch volumes), `image :pyfix` (= `:check` + a `python`
symlink, equivalent to the fixed Dockerfile line).

```
POST /api/ext/op/install {"package_name":"tts_webui_extension.kitten_tts"}
  -> 409 needs: gpu_choice, proposed CPU (nothing installed)
POST /api/ext/op/install {"package_name":"tts_webui_extension.kitten_tts","gpu":"CPU"}
```

(`gpu` takes the label, `CPU`; the row name `cpu` is refused with 400.)

- First attempt was cancelled when my client dropped the stream (the documented behaviour); the second ran to
  `Successfully set up virtual environment for Kitten TTS (uv) with dependencies`, `{"done":true,"ok":true}`.
- Step 1 of 2 (uv platform): downloaded `uv 0.13.6` from GitHub (checksum matched), `cpython-3.10.11` through uv, ffmpeg
  8.1.3 tools; `platforms/uv/platform.json` = `uv 0.13.6, python 3.10.11, gpu CPU`. Step 2: the venv with a CPU torch.
  On disk: `.venvs` 1.7 GB, `platforms` 356 MB. Not timed (a few minutes). After it, the 40 `uv` entries say `ready via uv`.
- `POST :3000/api/go-data/engines/tts_webui_extension.kitten_tts/open` (JSON + `Origin`): `starting`, then `ready`; the
  engine page `:7770/tts_webui_extension.kitten_tts/` answers 200 and its Gradio config has `api_name kitten_tts`.
- Clip: `gradio_client` `predict("KittenML/kitten-tts-nano-0.2", text, "expr-voice-2-f", api_name="/kitten_tts")` downloaded
  the model from Hugging Face and returned a 24 kHz WAV of 6.95 s in 9.7 s. **Model download from the runner worked.**
- **History: not verified.** The clip stayed in `/tmp/gradio/...` and `outputs/` stayed empty (checked in the container
  and the bind mount). The Kitten extension's own page (`tts_webui_extension/kitten_tts/main.py`) has no save step and the
  `Save Ogg` decorator did not fire on it, so nothing was there for History to list. I also tried the Go `openai` route
  (Activate API through `gradio_client`, restart, reopen the engine, `POST :7778/v1/audio/speech` with model
  `kitten-tts`): `500 Model kitten-tts not found in TTS adapter registry`, because an engine opened from the Vite page is
  not told `OPENAI_PROXY_HOST` (that comes from the Gradio-side extension launcher). Not chased further. The `onnx`
  platform (Go saves the clip) is Windows x64 only, so it is no help on Linux.
- Shutdown with the engine running: `docker stop` 4.7 s, exit 0, log `launcher exiting: stopping the app`, engine
  `exited code=0`.

### 4. Hardening notes

**There are none.** I searched `Dockerfile`, `docker-compose.yml`, `.dockerignore`, the workflows, `README.md`, and
`documentation/core-rewrite/` and `core/internal/*/INTEGRATION.md` for `non-root`, `USER`, `loopback`, `harden`: the only
loopback text is D9/D17/D20 about the launcher and port 3000. So the image cannot "not match" a note; what it does:

| Expectation named in the task | Image / compose as merged |
|---|---|
| Non-root | **No.** No `USER`; processes and files (bind mounts `data/`, `outputs/`, `favorites/` come out root-owned) run as uid 0 |
| Loopback publishing | **No.** Compose publishes `7770`, `3000`, `7778` on every host interface (`0.0.0.0`). Gradio on 7770 has no password and its router does not check `Host` (`Host: evil.example` gets 200). The launcher (7700, the token-protected one that starts processes) is not published, which is correct |

Not changed here: both would change who can use an existing deployment (bind mounts owned by root today). Suggested:
`ports: "127.0.0.1:7770:7770"`, a numeric `USER` plus `chown` of the volumes, and an `.env`/README note.

## Other findings (not fixed)

1. **Volumes lose installs.** Compose mounts `data/`, `outputs/`, `favorites/` only. `.venvs/`, `platforms/` (uv, Python, tools),
   `.env`, `config.json` live in the container's writable layer: an image update or `docker compose up --force-recreate`
   drops every installed engine (1.7 GB + 356 MB for one) and the settings.
2. **`uv` on PATH is not what `requires: uv` uses.** On the `system` platform the 40 `uv` entries show `missing` and
   "install the uv platform: uv 0.13.6, Python 3.10.11 and torch for the GPU you choose": the install downloads uv and
   Python again into `platforms/uv` instead of using the image's `uv==0.13.6`. The `uv` in the image serves only
   `Command("uv")` of the system platform. Fine if intended, but then the Dockerfile comment ("the system platform runs
   entries with the image's Python and this uv") should say entries that need `uv` get their own copy, and the image
   needs outbound access to GitHub, Hugging Face and PyPI at run time.
3. **CI only builds on push to `main`** (`publish-docker-image.yaml`); no PR build, no start check. Both defects above
   would have been caught by a PR job that builds and runs `curl :7770` and `curl :3000` (CPU only is enough, Gradio
   starts without a GPU). Not added: I cannot run workflows from here.
4. `COPY . /app/tts-webui` comes before `pip install -r requirements.txt`, so any source change reinstalls all
   requirements (84 s of the cold build here, more with a slower network). Copying `requirements.txt` first would keep that layer.
5. `documentation/core-rewrite/CONTRACTS.md:291` is stale: "Docker runs `python3 server.py --docker` and never uses the
   launcher". The image now runs the launcher.
6. The compose `restart: unless-stopped` plus `container_name` were not exercised (no recreate test).

## Not run

- GPU: no NVIDIA runtime, so `deploy.resources.reservations.devices` was removed and the CUDA row, `torch.cuda`, xformers
  and the NVIDIA install row were not run. The image printed `WARNING: The NVIDIA Driver was not detected` and ran CPU.
- The published image `ghcr.io/rsxdalv/tts-webui:main` (not pulled; the locally built tag was used). The GitHub
  workflows (`publish-docker-image`, `update-full-repo-zip`, `test-pip-install`, `core.yaml`) and
  `core/scripts/ci/install-host.sh` were read but **not run**.
- Pulling base images from Docker Hub directly (429, mirror used), a build on a network without the proxy and its CA.
- `linux/arm64`, Windows/macOS Docker Desktop, rootless Docker.
- A browser: the Vite pages and the launcher page were fetched with curl/python only; the Extensions page UI was not
  opened. Gradio tabs were not clicked (only `/config`, `api_name` calls through `gradio_client`).
- History (above), the Go `openai` route to a real engine, the audiocpp and onnx platforms (Windows only), removing an
  engine, a second start on a populated volume, `TTS_PORT`/`UI_PORT`/`OPENAI_API_PORT` overrides, and the commented
  `NVIDIA_DISABLE_REQUIRE` option.
