# The Go launcher: it runs the app (router, Gradio, React) and replaces `python server.py`.
FROM golang:1.27 AS launcher
WORKDIR /src
COPY core/ ./core/
RUN cd core && CGO_ENABLED=0 go build -trimpath -o /out/tts-webui ./cmd/tts-webui

# Python 3.10 w/ Nvidia Cuda
FROM nvidia/cuda:12.8.0-devel-ubuntu22.04 AS env_base

# Install Pre-reqs
RUN apt-get update && apt-get install --no-install-recommends -y \
    git vim nano build-essential python3-dev python3-venv python3-pip gcc g++ ffmpeg

ENV NODE_VERSION=22.9.0
RUN apt-get update && apt install -y curl
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
ENV NVM_DIR=/root/.nvm
RUN . "$NVM_DIR/nvm.sh" && nvm install ${NODE_VERSION}
RUN . "$NVM_DIR/nvm.sh" && nvm use v${NODE_VERSION}
RUN . "$NVM_DIR/nvm.sh" && nvm alias default v${NODE_VERSION}
ENV PATH="/root/.nvm/versions/node/v${NODE_VERSION}/bin/:${PATH}"
RUN node --version
RUN npm --version

# Install uv
# ADD --chmod=755 https://astral.sh/uv/install.sh /install.sh
# RUN /install.sh && rm /install.sh

# Define PyTorch version
ENV TORCH_VERSION=2.11.0

ENV PATH="/root/.cargo/bin:$PATH"
RUN pip install --no-cache-dir --upgrade pip setuptools wheel
RUN pip install --no-cache-dir setuptools torch==$TORCH_VERSION torchvision torchaudio==$TORCH_VERSION xformers==0.0.35 --index-url https://download.pytorch.org/whl/cu128

# Set working directory
WORKDIR /app

# Clone the repo
RUN git clone https://github.com/rsxdalv/tts-webui.git /app/tts-webui

# Set working directory to the cloned repo
WORKDIR /app/tts-webui

# Install all requirements
RUN pip3 install --no-cache-dir torch==$TORCH_VERSION -r requirements.txt
# RUN pip install --no-cache-dir --verbose torch==$TORCH_VERSION -r requirements.txt
RUN pip install "tts-webui-extension.rvc>=0.0.6" --extra-index-url https://tts-webui.github.io/extensions-index/
RUN pip install "tts-webui-extension.styletts2>=0.1.0" --extra-index-url https://tts-webui.github.io/extensions-index/


# SQLite is included with Python, no additional setup needed
# Build the React UI
RUN cd react-ui && npm install && npm run build:static

# The launcher starts the app on 0.0.0.0:7770 with the image's own Python and Node
COPY --from=launcher /out/tts-webui /usr/local/bin/tts-webui

# Run the server
CMD ["tts-webui", "launch", "--root", "/app/tts-webui", "--docker", "--start", "--system-python", "--no-browser"]
