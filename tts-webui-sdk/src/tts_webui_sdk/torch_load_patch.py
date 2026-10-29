"""
torch_load_patch: monkeypatches torch.load to always use weights_only=False.

This addresses the PyTorch 2.6 behaviour change where weights_only defaulted
to True, breaking most TTS model checkpoints.

Also exports torch_clear_memory() for freeing GPU/CPU memory between
generations.
"""

import gc
import logging


# ---------------------------------------------------------------------------
# torch_load_patch
# ---------------------------------------------------------------------------

try:
    import torch as _torch

    _original_torch_load = _torch.load

    def _patched_torch_load(*args, **kwargs):
        if "weights_only" not in kwargs:
            kwargs["weights_only"] = False
        return _original_torch_load(*args, **kwargs)

    def apply_torch_load_patch() -> None:
        """Monkeypatch torch.load to always set weights_only=False."""
        _torch.load = _patched_torch_load
        logging.info("Applied torch.load monkeypatch (weights_only=False)")

    def restore_original_torch_load() -> None:
        """Restore the original torch.load."""
        _torch.load = _original_torch_load
        logging.info("Restored original torch.load")

except ImportError:
    def apply_torch_load_patch() -> None:  # type: ignore[misc]
        logging.warning("torch not installed; torch_load_patch has no effect")

    def restore_original_torch_load() -> None:  # type: ignore[misc]
        pass


# ---------------------------------------------------------------------------
# torch_clear_memory
# ---------------------------------------------------------------------------

def torch_clear_memory() -> None:
    """Free GPU cache and run garbage collection."""
    try:
        import torch
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
    except Exception:
        pass
    gc.collect()
