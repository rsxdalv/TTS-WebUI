# The Go launcher: it runs the app (router, Gradio, React) and replaces `python server.py`.
FROM golang:1.27 AS launcher
WORKDIR /src
COPY core/ ./core/
RUN cd core && CGO_ENABLED=0 go build -trimpath -o /out/tts-webui ./cmd/tts-webui

# The React UI export (react-ui/out). Node is needed to build it, not to run the app.
FROM node:22 AS react
WORKDIR /src/react-ui
COPY react-ui/ ./
RUN npm install && npm run build:static

# Python 3.10 w/ Nvidia Cuda
FROM nvidia/cuda:12.8.0-devel-ubuntu22.04 AS env_base

# Install Pre-reqs. python-is-python3: the system platform starts "python" from PATH, and Ubuntu 22.04 only has
# python3 (without it the launcher says "python was not found on PATH" and Gradio never starts).
RUN apt-get update && apt-get install --no-install-recommends -y \
    git vim nano build-essential python3-dev python3-venv python3-pip python-is-python3 gcc g++ ffmpeg


# uv, at the version the uv platform pins (core/internal/platform/uv/pin.go): the system
# platform runs entries with the image's Python and this uv, from PATH.
RUN pip install --no-cache-dir uv==0.13.6

# Define PyTorch version
ENV TORCH_VERSION=2.11.0

ENV PATH="/root/.cargo/bin:$PATH"
RUN pip install --no-cache-dir --upgrade pip setuptools wheel
RUN pip install --no-cache-dir setuptools torch==$TORCH_VERSION torchvision torchaudio==$TORCH_VERSION xformers==0.0.35 --index-url https://download.pytorch.org/whl/cu128

# The app runs as this user, not as root (docker/entrypoint.sh drops root at start).
# The container publishes network services and runs model code, and docker-compose
# bind-mounts host folders into it: a compromise as root would write to them as root.
RUN useradd --create-home --uid 1000 appuser

# Set working directory
WORKDIR /app
RUN mkdir -p /app/tts-webui && chown appuser:appuser /app/tts-webui

# The sources of the commit being built (the archive of D32 is the same files), not a fresh clone of main.
COPY --chown=appuser:appuser . /app/tts-webui
WORKDIR /app/tts-webui

# Install all requirements
RUN pip3 install --no-cache-dir torch==$TORCH_VERSION -r requirements.txt
# RUN pip install --no-cache-dir --verbose torch==$TORCH_VERSION -r requirements.txt
RUN pip install "tts-webui-extension.rvc>=0.0.6" --extra-index-url https://tts-webui.github.io/extensions-index/
RUN pip install "tts-webui-extension.styletts2>=0.1.0" --extra-index-url https://tts-webui.github.io/extensions-index/


# The React UI export built above
COPY --from=react --chown=appuser:appuser /src/react-ui/out /app/tts-webui/react-ui/out

# The launcher starts the app on 0.0.0.0:7770 with the image's own Python
COPY --from=launcher /out/tts-webui /usr/local/bin/tts-webui

# Starts as root only to hand root-owned files in the bind-mounted folders to appuser (an
# update from an image that ran as root), then runs the launcher as appuser. It chains
# the base image's own entrypoint. TTS_WEBUI_RUN_AS_ROOT=1 keeps the old behaviour.
# pip falls back to a user install (~/.local) for the extensions installed at run time.
COPY docker/entrypoint.sh /usr/local/bin/tts-docker-entrypoint
RUN chmod 0755 /usr/local/bin/tts-docker-entrypoint
ENV PATH="/home/appuser/.local/bin:$PATH"
ENTRYPOINT ["/opt/nvidia/nvidia_entrypoint.sh", "/usr/local/bin/tts-docker-entrypoint"]

# Run the server
CMD ["tts-webui", "launch", "--root", "/app/tts-webui", "--docker", "--start", "--system-python", "--no-browser"]
