# Manual Installation Guide

## Installing Prerequisites

This project has a fair number of prerequisites, therefore we suggest using conda to install those:

Prerequisites:

- git
- PyTorch
- Python 3.10 or 3.11 (3.12 not supported yet)
- ffmpeg (with vorbis support)
- (Optional) NodeJS 22.9.0 for React UI
- (Optional) PostgreSQL 16.4+ for database support

### Conda requirements

- Install [conda](https://docs.conda.io/projects/conda/en/latest/user-guide/install/index.html)
- (Windows) Install [Visual Studio Build Tools](https://visualstudio.microsoft.com/visual-cpp-build-tools/)
- (Mac/Linux) GCC and other build tools (usually pre-installed)

1. Set up a conda environment:

   ```bash
   conda create -n venv
   ```

2. Install required packages:
   ```bash
   conda install -y -c conda-forge git \
       python=3.10.11 \
       conda-forge::nodejs=22.9.0 \
       conda conda-forge::uv=0.4.17 \
       ninja \
       conda-forge::postgresql=16.4 \
       conda-forge::vswhere \
       "conda-forge::ffmpeg=4.4.2[build=lgpl*]"
   ```
   (One line in case of copy-paste issues)
   ```bash
   conda install -y -c conda-forge git python=3.10.11 conda-forge::nodejs=22.9.0 conda conda-forge::uv=0.4.17 ninja conda-forge::postgresql=16.4 conda-forge::vswhere "conda-forge::ffmpeg=4.4.2[build=lgpl*]"
   ```

Note: Python 3.12 is not supported as of now. Python 3.11 works for many models, but not all.

### Without conda

1. git - [Windows](https://git-scm.com/download/win), [Mac](https://git-scm.com/download/mac), [Linux](https://git-scm.com/download/linux)

2. Python 3.10 or 3.11 - [Windows](https://www.python.org/downloads/windows/), [Mac](https://www.python.org/downloads/macos/), [Linux](https://www.python.org/downloads/source/)

3. NodeJS 22.9.0 - [Windows/Mac/Linux](https://nodejs.org/en/download/)

4. ffmpeg (with vorbis support) - [Windows](https://ffmpeg.org/download.html#build-windows), [Mac](https://ffmpeg.org/download.html#build-mac), [Linux](https://ffmpeg.org/download.html#build-linux)

5. (Optional) PostgreSQL 16.4+ - [Windows](https://www.postgresql.org/download/windows/), [Mac](https://www.postgresql.org/download/macosx/), [Linux](https://www.postgresql.org/download/linux/)

## Project Installation

### 0. Virtual Environment

It is recommended to use a virtual environment. You can use `venv` (comes with Python) or `conda` (recommended).

```bash
# Using venv
python -m venv venv
source venv/bin/activate  # On Windows use `venv\Scripts\activate`
```

```bash
# Using conda
conda create -n venv python=3.10
conda activate venv
```

### 1. PyTorch Installation

Choose the appropriate PyTorch installation for your hardware:

- **CPU/Mac**:

  ```bash
  pip install -U torch==2.7.0+cpu torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu
  ```

- **CUDA**:

  ```bash
  pip install -U torch==2.7.0+cu124 torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124
  ```

- **ROCM on Linux**:
  ```bash
  pip install -U torch==2.7.0 torchvision torchaudio --index-url https://download.pytorch.org/whl/rocm6.0
  ```

### 2. Project Setup

1. Clone the repository and install the requirements:

```bash
git clone https://github.com/rsxdalv/tts-webui.git
cd tts-webui
pip install -r requirements.txt
```

### 3. (Optional) Frontend Setup

Build the React app:

```bash
cd react-ui && npm install && npm run build
```

### 4. (Optional) Database Setup

Set up the database (optional):

```bash
node installer_scripts/js/applyDatabaseConfig.js
```

### 5. Start Server

Run the server:

```bash
python server.py
# or
python server.py --no-react
# or
python server.py --no-react --no-database
```

### 6. (Optional) Default Extensions

To install the default extensions, run and restart the server:

```bash
pip install -r requirements.txt \
   tts-webui-extension.bark_voice_clone>=0.0.1 \
   tts-webui-extension.rvc>=0.0.3 \
   tts-webui-extension.styletts2>=0.1.0 \
   tts-webui-extension.stable_audio>=0.1.1 \
   --extra-index-url https://tts-webui.github.io/extensions-index/
```

(One line in case of copy-paste issues)

```bash
pip install -r requirements.txt tts-webui-extension.bark_voice_clone>=0.0.1 tts-webui-extension.rvc>=0.0.3 tts-webui-extension.styletts2>=0.1.0 tts-webui-extension.stable_audio>=0.1.1 hydra-core==1.3.2 nvidia-ml-py --extra-index-url https://tts-webui.github.io/extensions-index/
```

In case of failures, try installing the extensions one by one.

```bash
pip install tts-webui-extension.bark_voice_clone>=0.0.1 --extra-index-url https://tts-webui.github.io/extensions-index/
pip install tts-webui-extension.rvc>=0.0.3 --extra-index-url https://tts-webui.github.io/extensions-index/
pip install tts-webui-extension.audiocraft>=0.0.2 --extra-index-url https://tts-webui.github.io/extensions-index/
pip install tts-webui-extension.styletts2>=0.1.0 --extra-index-url https://tts-webui.github.io/extensions-index/
pip install tts-webui-extension.vall_e_x>=0.1.0 --extra-index-url https://tts-webui.github.io/extensions-index/
pip install tts-webui-extension.stable_audio>=0.1.1 --extra-index-url https://tts-webui.github.io/extensions-index/
```

### 7. Updates

To update the installation, navigate to the project directory and run:

```bash
git pull
pip install -r requirements.txt
cd react-ui && npm install && npm run build
```

# Development Setup

## React UI Development Setup

If you want to work on the React UI:

1. Install Node.js (if not already installed with conda)

2. Install React dependencies:

   ```bash
   cd react-ui
   npm install
   ```

3. For development, run React in development mode:

   ```bash
   npm start
   ```

4. In a separate terminal, run the Python server:
   ```bash
   python server.py
   ```
   or use the `start_tts_webui` script
