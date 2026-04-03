# TTS-WebUI Dockerfile

ARG CUDA_VERSION=12.8

FROM nvidia/cuda:${CUDA_VERSION}-devel-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.10 \
    python3-pip \
    ffmpeg \
    libsndfile1 \
    git \
    wget \
    unzip \
    curl \
    && rm -rf /var/lib/apt/lists/*

# install uv
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:$PATH"

WORKDIR /app

# set torch version
ENV TORCH_VERSION=2.7.0

# install PyTorch with CUDA support
RUN pip install torch==$TORCH_VERSION torchvision torchaudio --index-url https://download.pytorch.org/whl/cu128

# install requirements
COPY requirements.txt .
RUN pip install -r requirements.txt

# install extensions
RUN pip install \
    "tts-webui-extension.bark_voice_clone>=0.0.2" \
    "tts-webui-extension.rvc>=0.0.5" \
    "tts-webui-extension.styletts2>=0.1.0" \
    "tts-webui-extension.stable_audio>=0.1.1" \
    --extra-index-url https://tts-webui.github.io/extensions-index/

# copy source
COPY . .

ENV PORT=7860
EXPOSE $PORT

CMD ["python", "-m", "tts_webui"]
