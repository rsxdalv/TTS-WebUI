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



import json
import os

JSON_ENV_FILE = "env_store.json"
DOTENV_FILE = ".env"

def load_env_store():
    if os.path.exists(JSON_ENV_FILE): 
        with open(JSON_ENV_FILE, "r") as f:
            return json.load(f)
    return {}

def save_env_store(data):
    with open(JSON_ENV_FILE, "w") as f:
        json.dump(data, f, indent=2)

def env_entry(name, value, comment="", null_if_empty=True):
    return f"# {comment}\n{'# ' if null_if_empty and not value else ''}{name}={value}\n"

def generate_dotenv_text(env_store: dict) -> str:
    text = (
        "# This file gets updated automatically from the JSON store.\n"
        "# Manual changes will be overwritten.\n\n"
    )

    for namespace, vars_dict in env_store.items():
        text += f"# --- {namespace.upper()} ---\n"
        for key, value in vars_dict.items():
            comment = f"{namespace}.{key}"
            text += env_entry(key, value, comment)
        text += "\n"
    return text

def write_dotenv_from_json(env_store):
    dotenv_text = generate_dotenv_text(env_store)
    with open(DOTENV_FILE, "w") as f:
        f.write(dotenv_text)

def update_dotenv(namespace: str, values: dict):
    env_store = load_env_store()
    env_store.setdefault(namespace, {})
    env_store[namespace].update(values)
    save_env_store(env_store)
    write_dotenv_from_json(env_store)

def delete_namespace(namespace: str):
    env_store = load_env_store()
    if namespace in env_store:
        del env_store[namespace]
        save_env_store(env_store)
        write_dotenv_from_json(env_store)