"""
Decorator helpers for TTS WebUI extensions.

All decorators operate on a result_dict convention:
  result_dict["audio_out"] = (sample_rate: int, audio_array: np.ndarray)
  result_dict["filename"]  = base filename (set by decorator_add_base_filename)
  result_dict["folder_root"] = output directory path
  result_dict["date"]     = datetime object
  result_dict["metadata"] = dict (set by decorator_save_metadata)
"""

import asyncio
import gc
import hashlib
import json
import logging
import os
import random
import time
from contextlib import contextmanager
from datetime import datetime

import numpy as np
from scipy.io.wavfile import write as write_wav

# ---------------------------------------------------------------------------
# Output helpers (inlined from tts_webui.utils.outputs.path)
# ---------------------------------------------------------------------------

_dir_cache: dict = {}


def _get_relative_output_path(result_dict: dict, *args: str) -> str:
    dir_ = result_dict["folder_root"]
    if dir_ not in _dir_cache:
        os.makedirs(dir_, exist_ok=True)
        _dir_cache[dir_] = True
    return os.path.join(dir_, *args)


def _get_relative_output_path_ext(result_dict: dict, ext: str) -> str:
    return _get_relative_output_path(result_dict, result_dict["filename"] + ext)


# ---------------------------------------------------------------------------
# Audio hash helper
# ---------------------------------------------------------------------------

def _audio_array_to_sha256(audio_array: np.ndarray) -> str:
    return hashlib.sha256(audio_array.tobytes()).hexdigest()


# ---------------------------------------------------------------------------
# Prompt-to-title helper
# ---------------------------------------------------------------------------

def _prompt_to_title(prompt: str) -> str:
    for ch in " :'\"\\.,()!?/\n":
        prompt = prompt.replace(ch, "_")
    return prompt.replace("__", "_")[:15]


# ---------------------------------------------------------------------------
# Seed helpers (torch-optional)
# ---------------------------------------------------------------------------

def _set_seed(seed: int = 0) -> int:
    original = seed
    try:
        import torch
        if seed == -1:
            torch.backends.cudnn.deterministic = False
            torch.backends.cudnn.benchmark = True
        else:
            if seed == 0:
                seed = random.randint(1, 2**32 - 1)
            torch.backends.cudnn.deterministic = True
            torch.backends.cudnn.benchmark = False
            torch.manual_seed(seed)
            try:
                torch.cuda.manual_seed_all(seed)
            except Exception:
                pass
    except ImportError:
        pass
    np.random.seed(seed % (2**32) if seed >= 0 else None)
    random.seed(seed)
    return original


@contextmanager
def _seed_context(seed: int):
    original = _set_seed(seed)
    yield
    _set_seed(original)


# ---------------------------------------------------------------------------
# Log generation helper
# ---------------------------------------------------------------------------

def _log_generation(params: dict) -> None:
    text = params.get("text", "")
    if text:
        text = text[:50] + "..." if len(text) > 50 else text
        print(f"Generating: '''{text}'''")
    _type = params.get("_type", "Params")
    filtered = {k: v for k, v in params.items() if k not in ("outputs", "_type", "text")}
    params_str = ",\n    ".join(f"{k}={repr(v) if not isinstance(v, dict) else 'dict'}"
                                for k, v in filtered.items())
    print(f"{_type.capitalize()}(\n    {params_str}\n)")


# ---------------------------------------------------------------------------
# InterruptionFlag
# ---------------------------------------------------------------------------

class InterruptionFlag:
    """Async-safe interruption flag for cancelling long TTS generations."""

    def __init__(self) -> None:
        self._interrupted = False
        self._ack_event = asyncio.Event()

    def interrupt(self) -> None:
        self._interrupted = True

    def reset(self) -> None:
        self._interrupted = False
        self._ack_event.clear()

    def is_interrupted(self) -> bool:
        return self._interrupted

    def acknowledge(self) -> None:
        self._ack_event.set()

    async def join(self, timeout: float | None = None) -> None:
        try:
            await asyncio.wait_for(self._ack_event.wait(), timeout)
        except asyncio.TimeoutError:
            raise RuntimeError("Timeout waiting for interruption acknowledgement.")


_interruption_flags: dict[str, InterruptionFlag] = {}


def get_interruption_flag(namespace: str) -> InterruptionFlag:
    return _interruption_flags.setdefault(namespace, InterruptionFlag())


# ---------------------------------------------------------------------------
# Base filename decorator
# ---------------------------------------------------------------------------

_output_path = "outputs"


def format_date_for_file(date: datetime) -> str:
    return date.strftime("%Y-%m-%d_%H-%M-%S")


def _create_filename(kwargs: dict, result_dict: dict) -> str:
    prompt = kwargs.get("text", "")
    is_long = result_dict.get("long", False)
    title = _prompt_to_title(prompt) + ("_long" if is_long else "")
    model = _prompt_to_title(kwargs.get("_type", ""))
    date = format_date_for_file(result_dict["date"])
    return f"{date}__{model}__{title}"


def _add_filename(kwargs: dict, result_dict: dict) -> dict:
    base = _create_filename(kwargs, result_dict)
    result_dict["filename"] = base
    result_dict["folder_root"] = os.path.join(_output_path, base)
    return result_dict


def decorator_add_base_filename(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        return _add_filename(kwargs, result_dict)
    return wrapper


def decorator_add_base_filename_generator(fn):
    def wrapper(*args, **kwargs):
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            yield _add_filename(kwargs, result_dict)
    return wrapper


def decorator_add_base_filename_generator_accumulated(fn):
    def wrapper(*args, **kwargs):
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            yield _add_filename(kwargs, result_dict)
    return wrapper


# ---------------------------------------------------------------------------
# Save WAV decorator
# ---------------------------------------------------------------------------

def _save_wav(result_dict: dict) -> None:
    sample_rate, audio_array = result_dict["audio_out"]
    path = _get_relative_output_path_ext(result_dict, ".wav")
    print("Saving generation to", path)
    write_wav(path, sample_rate, audio_array)


def decorator_save_wav(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        _save_wav(result_dict)
        return result_dict
    return wrapper


def decorator_save_wav_generator(fn):
    def wrapper(*args, **kwargs):
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            _save_wav(result_dict)
            yield result_dict
    return wrapper


def decorator_save_wav_generator_accumulated(fn):
    def wrapper(*args, **kwargs):
        save_each = kwargs.get("generator_save_each", False)
        accumulated = None
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            if save_each:
                _save_wav(result_dict)
                continue
            if accumulated is None:
                accumulated = result_dict
            else:
                accumulated = {
                    **accumulated,
                    **result_dict,
                    "audio_out": (
                        accumulated["audio_out"][0],
                        np.concatenate([accumulated["audio_out"][1], result_dict["audio_out"][1]]),
                    ),
                }
            yield result_dict
        if not save_each and accumulated is not None:
            _save_wav(accumulated)
    return wrapper


# ---------------------------------------------------------------------------
# Save metadata decorator
# ---------------------------------------------------------------------------

def _add_metadata(result_dict: dict, kwargs: dict) -> dict:
    result_dict["metadata"] = {
        "_version": "0.0.1",
        "_hash_version": "0.0.2",
        **kwargs,
        "outputs": None,
        "date": str(result_dict["date"]),
        "hash": _audio_array_to_sha256(result_dict["audio_out"][1]),
    }
    return result_dict


def _save_metadata(result_dict: dict, kwargs: dict) -> dict:
    path = _get_relative_output_path_ext(result_dict, ".json")
    print("Saving metadata to", path)
    with open(path, "w") as f:
        json.dump(
            result_dict["metadata"],
            f,
            indent=2,
            skipkeys=True,
            default=lambda o: f"<<non-serializable: {type(o).__qualname__}>>",
        )
    return result_dict


def decorator_save_metadata(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        result_dict = _add_metadata(result_dict, kwargs)
        return _save_metadata(result_dict, kwargs)
    return wrapper


def decorator_save_metadata_generator(fn):
    def wrapper(*args, **kwargs):
        last = None
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            result_dict = _add_metadata(result_dict, kwargs)
            last = result_dict
            yield result_dict
        if last is not None:
            _save_metadata(last, kwargs)
    return wrapper


# ---------------------------------------------------------------------------
# Log generation decorator
# ---------------------------------------------------------------------------

def decorator_log_generation(fn):
    def wrapper(*args, **kwargs):
        _log_generation(kwargs)
        return fn(*args, **kwargs)
    return wrapper


def decorator_log_generation_generator(fn):
    def wrapper(*args, **kwargs):
        _log_generation(kwargs)
        yield from fn(*args, **kwargs)
    return wrapper


# ---------------------------------------------------------------------------
# Seed decorator
# ---------------------------------------------------------------------------

def decorator_apply_torch_seed(fn):
    def wrapper(*args, **kwargs):
        with _seed_context(int(kwargs.get("seed", -1))):
            return fn(*args, **kwargs)
    return wrapper


def decorator_apply_torch_seed_generator(fn):
    def wrapper(*args, **kwargs):
        with _seed_context(int(kwargs.get("seed", -1))):
            yield from fn(*args, **kwargs)
    return wrapper
