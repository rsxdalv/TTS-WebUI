"""Output path conventions for TTS WebUI generations."""

import os

_dir_cache: dict = {}


def _get_relative_output_path(result_dict: dict, *args: str) -> str:
    """Return the path to a file within the generation's output folder."""
    dir_ = result_dict["folder_root"]
    if dir_ not in _dir_cache:
        os.makedirs(dir_, exist_ok=True)
        _dir_cache[dir_] = True
    return os.path.join(dir_, *args)


def get_relative_output_path_ext(result_dict: dict, ext: str) -> str:
    """Return the path to a generation file with the given extension (.wav, .json, etc.)."""
    return _get_relative_output_path(result_dict, result_dict["filename"] + ext)
