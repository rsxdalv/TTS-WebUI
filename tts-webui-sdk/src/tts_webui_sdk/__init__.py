"""TTS WebUI SDK — decorators, output helpers, and utilities for extension authors."""

from .decorators import (
    decorator_save_wav,
    decorator_save_wav_generator,
    decorator_save_wav_generator_accumulated,
    decorator_save_metadata,
    decorator_save_metadata_generator,
    decorator_add_base_filename,
    decorator_add_base_filename_generator,
    decorator_add_base_filename_generator_accumulated,
    decorator_log_generation,
    decorator_log_generation_generator,
    decorator_apply_torch_seed,
    decorator_apply_torch_seed_generator,
    get_interruption_flag,
    InterruptionFlag,
)
from .output import get_relative_output_path_ext
from .torch_load_patch import apply_torch_load_patch, restore_original_torch_load, torch_clear_memory

__all__ = [
    "decorator_save_wav",
    "decorator_save_wav_generator",
    "decorator_save_wav_generator_accumulated",
    "decorator_save_metadata",
    "decorator_save_metadata_generator",
    "decorator_add_base_filename",
    "decorator_add_base_filename_generator",
    "decorator_add_base_filename_generator_accumulated",
    "decorator_log_generation",
    "decorator_log_generation_generator",
    "decorator_apply_torch_seed",
    "decorator_apply_torch_seed_generator",
    "get_interruption_flag",
    "InterruptionFlag",
    "get_relative_output_path_ext",
    "apply_torch_load_patch",
    "restore_original_torch_load",
    "torch_clear_memory",
]
