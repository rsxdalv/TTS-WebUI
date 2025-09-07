# Manual Installation Guide

> **Warning**: These instructions may be outdated compared to the automated installer but serve as reference for debugging or understanding what the installer does. Hopefully they can be a basis for supporting new platforms, such as AMD/Intel.

## Prerequisites

- Install [conda](https://docs.conda.io/projects/conda/en/latest/user-guide/install/index.html)
- (Windows) Install [Visual Studio Build Tools](https://visualstudio.microsoft.com/visual-cpp-build-tools/)

## Basic Environment Setup

1. Set up a conda environment:
   ```bash
   conda create -n venv
   ```

2. Install required packages:
   ```bash
   conda install -y -c conda-forge git python=3.10.11 conda-forge::nodejs=22.9.0 conda conda-forge::uv=0.4.17 ninja conda-forge::postgresql=16.4 conda-forge::vswhere "conda-forge::ffmpeg=4.4.2[build=lgpl*]"
   ```

Note: Python 3.12 is not supported as of now. Python 3.11 works for many models, but not all.

## Installation Options

You can choose one of the following installation methods:

### Option A: Using the Installer Script

1. Activate the conda environment:
   ```bash
   conda activate venv
   ```

2. Run the installer script:
   ```bash
   node installer_scripts\init_app.js
   ```

3. Start the server:
   ```bash
   python server.py
   ```

### Option B: Manual Installation

#### 1. PyTorch Installation

Choose the appropriate PyTorch installation for your hardware:

- **CPU/Mac**:
  ```bash
  pip install -U torch==2.6.0+cpu torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu
  ```

- **CUDA**:
  ```bash
  pip install -U torch==2.6.0+cu124 torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124
  ```

- **ROCM on Linux**:
  ```bash
  pip install -U torch==2.6.0 torchvision torchaudio --index-url https://download.pytorch.org/whl/rocm6.0
  ```

#### 2. Project Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/rsxdalv/tts-webui.git
   ```

2. Install the requirements:

   You can install each requirement file individually:
   ```bash
   pip install -r requirements.txt
   pip install git+https://github.com/rsxdalv/extension_bark_voice_clone@main
   pip install git+https://github.com/rsxdalv/extension_rvc@main
   pip install git+https://github.com/rsxdalv/extension_audiocraft@main
   pip install git+https://github.com/rsxdalv/extension_styletts2@main
   pip install git+https://github.com/rsxdalv/extension_vall_e_x@main
   pip install git+https://github.com/rsxdalv/extension_maha_tts@main
   pip install git+https://github.com/rsxdalv/extension_stable_audio@main
   pip install hydra-core==1.3.2
   pip install nvidia-ml-py
   ```

   Or optionally install all requirements at once (may fail on some systems):
   ```bash
   pip install -r requirements.txt git+https://github.com/rsxdalv/extension_audiocraft@main git+https://github.com/rsxdalv/extension_bark_voice_clone@main git+https://github.com/rsxdalv/extension_maha_tts@main git+https://github.com/rsxdalv/extension_rvc@main git+https://github.com/rsxdalv/extension_stable_audio@main git+https://github.com/rsxdalv/extension_styletts2@main git+https://github.com/rsxdalv/extension_vall_e_x@main hydra-core==1.3.2 nvidia-ml-py
   ```

   > Note: This list might not be up to date. Check the [Dockerfile](https://github.com/rsxdalv/tts-webui/blob/main/Dockerfile#L39-L40) for the latest requirements.

#### 3. Frontend Setup

Build the React app:
```bash
cd react-ui && npm install && npm run build
```

#### 4. Optional Database Setup

Set up the database (optional):
```bash
node installer_scripts/js/applyDatabaseConfig.js
```

#### 5. Start Server

Run the server:
```bash
python server.py
```

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
