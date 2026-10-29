"""
Utility module to monkeypatch torch.load to always use weights_only=False.
This addresses the issue with PyTorch 2.6 changing the default value of weights_only from False to True.
"""

import functools
import logging


def apply_torch_load_patch():
    """Apply the monkeypatch to torch.load. No-op if torch is not installed (P5)."""
    try:
        import torch  # lazy — torch is optional
    except ImportError:
        logging.debug("torch not available; skipping torch.load patch")
        return

    original_torch_load = torch.load

    @functools.wraps(original_torch_load)
    def patched_torch_load(*args, **kwargs):
        """
        Wrapper for torch.load that always sets weights_only=False.
        This addresses the issue with PyTorch 2.6 changing the default value of weights_only from False to True.
        """
        if "weights_only" not in kwargs:
            kwargs["weights_only"] = False
        return original_torch_load(*args, **kwargs)

    torch.load = patched_torch_load
    logging.debug("torch.load patched to default weights_only=False")


def restore_original_torch_load():
    """Restore the original torch.load. No-op if torch is not installed (P5)."""
    try:
        import torch  # lazy — torch is optional
        torch.load = torch.load.__wrapped__  # type: ignore[attr-defined]
    except (ImportError, AttributeError):
        pass


# Re-exported from tts-webui-sdk (P2)
try:
    from tts_webui_sdk.torch_load_patch import (  # noqa: F401
        apply_torch_load_patch as _sdk_apply_torch_load_patch,
        torch_clear_memory,
    )
except ImportError:
    pass
