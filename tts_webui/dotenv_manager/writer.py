"""The default .env text, for server.py's init() when .env is missing.

The Go core creates and rewrites .env when it runs the app (core/internal/
serverpy/appenv: EnsureEnv, SaveModelLocations, which the Model Location tab
reaches through bridge.env); this module is the legacy server.py path's copy.
"""

import os


def env_entry(name, value, comment, null_if_empty=True):
    return f"# {comment}\n{'# ' if null_if_empty and not value else ''}{name}={value}\n"


def generate_env(
    *,
    model_location_hf_env_var: str = "",
    model_location_hf_env_var2: str = "",
    model_location_th_home: str = "",
    model_location_th_xdg: str = "",
):
    env = """
# This file gets updated automatically from the UI

# If you wish to manually specify any ENV variables, please do so in the .env.user file
# The variables in .env.user will take PRIORITY!

# Defaults
USE_TF=0 # Disable Tensorflow for transformers

# RVC Defaults
weight_root=data/models/rvc/checkpoints # Root directory for RVC model weights
weight_uvr5_root=data/models/rvc/uvr5_weights # Root directory for RVC model weights (UVR5)
index_root=data/models/rvc/checkpoints # Root directory for RVC model indices
outside_index_root=data/models/rvc/checkpoints # Root directory for RVC model indices (outside)
rmvpe_root=data/models/rvc/rmvpe # Root directory for RVC model RMVPE

"""
    model_location_hf_env_var = model_location_hf_env_var or os.environ.get(
        "HUGGINGFACE_HUB_CACHE", ""
    )
    model_location_hf_env_var2 = model_location_hf_env_var2 or os.environ.get(
        "HF_HOME", ""
    )
    model_location_th_home = model_location_th_home or os.environ.get("TORCH_HOME", "")
    model_location_th_xdg = model_location_th_xdg or os.environ.get(
        "XDG_CACHE_HOME", ""
    )

    env += env_entry(
        "HUGGINGFACE_HUB_CACHE",
        model_location_hf_env_var,
        "Environment variable for HuggingFace model location",
    )
    env += env_entry(
        "HF_HOME",
        model_location_hf_env_var2,
        "Environment variable for HuggingFace model location (alternative)",
    )
    env += env_entry(
        "TORCH_HOME",
        model_location_th_home,
        "Default location for Torch Hub models",
    )
    env += env_entry(
        "XDG_CACHE_HOME",
        model_location_th_xdg,
        "Default location for Torch Hub models (alternative)",
    )

    env += "\n"

    # CUDA_VISIBLE_DEVICES
    # env += env_entry(
    #     "CUDA_VISIBLE_DEVICES",
    #     "0",
    #     "CUDA device to use (0,1,2,3, etc.)",
    #     null=False,
    # )

    return env


def write_env(text: str):
    with open(".env", "w") as outfile:
        outfile.write(text)
