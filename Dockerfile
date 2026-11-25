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

# Install Pre-reqs
RUN apt-get update && apt-get install --no-install-recommends -y \
    git vim nano build-essential python3-dev python3-venv python3-pip gcc g++ ffmpeg


# uv, at the version the uv platform pins (core/internal/platform/uv/pin.go): the system
# platform runs entries with the image's Python and this uv, from PATH.
RUN pip install --no-cache-dir uv==0.13.6

# Define PyTorch version
ENV TORCH_VERSION=2.11.0

ENV PATH="/root/.cargo/bin:$PATH"
RUN pip install --no-cache-dir --upgrade pip setuptools wheel
RUN pip install --no-cache-dir setuptools torch==$TORCH_VERSION torchvision torchaudio==$TORCH_VERSION xformers==0.0.35 --index-url https://download.pytorch.org/whl/cu128

# Set working directory
WORKDIR /app

# The sources of the commit being built (the archive of D32 is the same files), not a fresh clone of main.
COPY . /app/tts-webui
WORKDIR /app/tts-webui

# Install all requirements
RUN pip3 install --no-cache-dir torch==$TORCH_VERSION -r requirements.txt
# RUN pip install --no-cache-dir --verbose torch==$TORCH_VERSION -r requirements.txt
RUN pip install "tts-webui-extension.rvc>=0.0.6" --extra-index-url https://tts-webui.github.io/extensions-index/
RUN pip install "tts-webui-extension.styletts2>=0.1.0" --extra-index-url https://tts-webui.github.io/extensions-index/


# The React UI export built above
COPY --from=react /src/react-ui/out /app/tts-webui/react-ui/out

# The launcher starts the app on 0.0.0.0:7770 with the image's own Python
COPY --from=launcher /out/tts-webui /usr/local/bin/tts-webui

# Run the server
CMD ["tts-webui", "launch", "--root", "/app/tts-webui", "--docker", "--start", "--system-python", "--no-browser"]
