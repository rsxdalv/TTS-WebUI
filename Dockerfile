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
ENV TORCH_VERSION=2.7.0

ENV PATH="/root/.cargo/bin:$PATH"
RUN pip install --no-cache-dir --upgrade pip setuptools wheel
# add xformers?
RUN pip install --no-cache-dir setuptools torch==$TORCH_VERSION torchvision torchaudio --index-url https://download.pytorch.org/whl/cu128

# Set working directory
WORKDIR /app

# Clone the repo
RUN git clone https://github.com/rsxdalv/tts-webui.git /app/tts-webui

# Set working directory to the cloned repo
WORKDIR /app/tts-webui

# Install all requirements
RUN pip3 install --no-cache-dir torch==$TORCH_VERSION -r requirements.txt
# RUN pip install --no-cache-dir --verbose torch==$TORCH_VERSION -r requirements.txt
RUN pip install "tts-webui-extension.bark_voice_clone>=0.0.2" --extra-index-url https://tts-webui.github.io/extensions-index/
RUN pip install "tts-webui-extension.rvc>=0.0.6" --extra-index-url https://tts-webui.github.io/extensions-index/
RUN pip install "tts-webui-extension.styletts2>=0.1.0" --extra-index-url https://tts-webui.github.io/extensions-index/
# RUN pip install "tts-webui-extension.stable_audio>=0.1.1" --extra-index-url https://tts-webui.github.io/extensions-index/


# SQLite is included with Python, no additional setup needed
# Build the React UI
RUN cd react-ui && npm install && npm run build

# Run the server
CMD python3 server.py --docker
