import json
from tts_webui.utils.audio_array_to_sha256 import audio_array_to_sha256
from tts_webui.utils.outputs.path import get_relative_output_path_ext


def _save_metadata_to_result(result_dict, kwargs):
    """Helper function to save metadata to a result dictionary."""
    path = get_relative_output_path_ext(result_dict, ".json")
    print("Saving metadata to", path)

    metadata = {
        "_version": "0.0.1",
        "_hash_version": "0.0.2",
        **kwargs,
        "outputs": None,
        "date": str(result_dict["date"]),
        "hash": audio_array_to_sha256(result_dict["audio_out"][1]),
        # **result_dict,
    }

    with open(path, "w") as outfile:
        json.dump(
            metadata,
            outfile,
            indent=2,
            skipkeys=True,
            default=lambda o: f"<<non-serializable: {type(o).__qualname__}>>",
        )

    result_dict["metadata"] = metadata
    return result_dict


def decorator_save_metadata(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        return _save_metadata_to_result(result_dict, kwargs)

    return wrapper


def decorator_save_metadata_generator(fn):
    def wrapper(*args, **kwargs):
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            yield _save_metadata_to_result(result_dict, kwargs)

    return wrapper
