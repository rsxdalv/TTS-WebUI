import os

TORCH_VERSION = "2.11.0"
CUDA_VERSION = "12.8"
CUDA_VERSION_TAG = "cu128"

ROCM_VERSION_TAG = {
    "2.6.0": "rocm6.2.4",
    "2.7.0": "rocm6.3",
    "2.7.1": "rocm6.3",
    "2.11.0": "rocm7.2",
}

PYTORCH_CHOICE_MAP = {
    "NVIDIA GPU": {
        "package": f"torch=={TORCH_VERSION}+{CUDA_VERSION_TAG} torchvision torchaudio=={TORCH_VERSION} xformers==0.0.35",
        "index_url": f"https://download.pytorch.org/whl/{CUDA_VERSION_TAG}",
    },
    "Custom (Manual torch install)": {
        "package": f"torch=={TORCH_VERSION}+{CUDA_VERSION_TAG} torchvision torchaudio=={TORCH_VERSION}",
        "index_url": f"https://download.pytorch.org/whl/{CUDA_VERSION_TAG}",
        "manual": True,
    },
    "Apple M Series Chip": {
        "package": f"torch=={TORCH_VERSION} torchvision torchaudio=={TORCH_VERSION}",
        "index_url": None,
    },
    "CPU": {
        "package": f"torch=={TORCH_VERSION}+cpu torchvision torchaudio=={TORCH_VERSION}",
        "index_url": "https://download.pytorch.org/whl/cpu",
    },
    "AMD GPU (ROCM, Linux only)": {
        "package": f"torch=={TORCH_VERSION} torchvision torchaudio=={TORCH_VERSION} xformers",
        "index_url": f"https://download.pytorch.org/whl/{ROCM_VERSION_TAG[TORCH_VERSION]}",
    },
    "Intel GPU (XPU)": {
        "package": f"torch=={TORCH_VERSION} torchvision torchaudio=={TORCH_VERSION}",
        "index_url": "https://download.pytorch.org/whl/test/xpu",
    },
}

GPU_FILE_PATH = os.path.join(
    os.path.dirname(__file__), "..", "..", "installer_scripts", ".gpu"
)


def get_gpu_choice():
    """Read the GPU choice from the saved file."""
    if os.path.exists(GPU_FILE_PATH):
        with open(GPU_FILE_PATH, "r") as f:
            return f.read().strip()
    else:
        raise FileNotFoundError(
            f"GPU choice file not found in {GPU_FILE_PATH}. Please run the installer to select your GPU."
        )


def _get_torch_sources():
    """
    Internal: Returns the torch package string and index URL based on GPU choice.
    Returns a dict with 'package' and 'index_url' keys, or None if unsupported.
    """
    gpu_choice = get_gpu_choice()

    if gpu_choice in PYTORCH_CHOICE_MAP:
        return PYTORCH_CHOICE_MAP[gpu_choice]

    raise ValueError(f"Unsupported GPU choice: {gpu_choice}")


def get_torch_command():
    """
    Returns the full pip install command string for torch based on GPU choice.
    Returns the command string, or the default NVIDIA CUDA command if no GPU choice is set.
    """
    sources = _get_torch_sources()

    package = sources["package"]
    index_url = sources["index_url"]

    if index_url:
        return f"{package} --index-url {index_url}"
    return package


TORCHCODEC_MAPPING = {
    "2.5.0": "0.1",
    "2.6.0": "0.2",
    "2.7.0": "0.5",
    "2.7.1": "0.5",
    "2.8.0": "0.8",
    "2.9.0": "0.9",
    "2.10.0": "0.10",
    "2.11.0": "0.11",
}


def get_torchcodec_command():
    """
    Returns the torchcodec package string with the correct version based on the installed torch version.
     Uses the TORCHCODEC_MAPPING to find the compatible torchcodec version for the detected torch version.
     Defaults to a safe CPU-only torchcodec if the torch version is not in the mapping.
    """

    return f"torchcodec=={TORCHCODEC_MAPPING[TORCH_VERSION]} --index-url https://download.pytorch.org/whl/cpu"
