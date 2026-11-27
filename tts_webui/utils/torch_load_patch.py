"""
Utility module to control the ``weights_only`` default of ``torch.load``.

PyTorch 2.6 changed the default of ``weights_only`` from ``False`` to ``True``,
which stops ``torch.load`` from unpickling arbitrary Python objects. Some older
checkpoints only load with ``weights_only=False``, so this module used to patch
``torch.load`` globally to always pass ``False``.

That is unsafe. ``torch.load(weights_only=False)`` executes code embedded in the
checkpoint, so every code path that takes a model name or path from a request
turns into remote code execution -- for example ``whisper_transcribe``, whose
``model_name`` argument reaches ``from_pretrained()`` unvalidated.

The global patch is therefore opt-in. Prefer ``safetensors`` checkpoints, or
wrap the specific load in ``unsafe_torch_load()`` when the file is known to be
trusted.
"""

import contextlib
import functools
import logging
import os

import torch

# Store the original torch.load function
original_torch_load = torch.load

UNSAFE_ENV_VAR = "TTS_WEBUI_UNSAFE_TORCH_LOAD"


@functools.wraps(original_torch_load)
def patched_torch_load(*args, **kwargs):
    """
    Wrapper for torch.load that defaults weights_only to False.

    Only installed when the operator explicitly opts in; see the module
    docstring for why.
    """
    # Explicitly set weights_only to False if not provided
    if "weights_only" not in kwargs:
        kwargs["weights_only"] = False

    return original_torch_load(*args, **kwargs)


def apply_torch_load_patch():
    """
    Apply the monkeypatch to torch.load, if the operator opted in.

    Set TTS_WEBUI_UNSAFE_TORCH_LOAD=1 to restore the old always-unsafe
    behaviour. Without it torch keeps its own safe default.
    """
    if os.environ.get(UNSAFE_ENV_VAR, "").strip().lower() not in (
        "1",
        "true",
        "yes",
    ):
        logging.info(
            "torch.load keeps weights_only=True (PyTorch default). "
            "Set %s=1 to load legacy pickled checkpoints.",
            UNSAFE_ENV_VAR,
        )
        return

    torch.load = patched_torch_load
    print(
        f"WARNING: {UNSAFE_ENV_VAR} is set. torch.load will unpickle arbitrary "
        "objects, so any checkpoint you load can execute code."
    )
    logging.warning(
        "Applied monkeypatch to torch.load to always use weights_only=False"
    )


def restore_original_torch_load():
    """
    Restore the original torch.load function.
    """
    torch.load = original_torch_load
    logging.info("Restored original torch.load function")


@contextlib.contextmanager
def unsafe_torch_load():
    """
    Temporarily allow pickled checkpoints for a single, trusted load.

    Use this instead of the global patch when a specific model is known to
    ship a legacy .bin/.pth and the path is not attacker-controlled.
    """
    previous = torch.load
    torch.load = patched_torch_load
    try:
        yield
    finally:
        torch.load = previous
