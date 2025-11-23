from tempfile import NamedTemporaryFile
import numpy as np
import json
from typing import Any, Dict, List, Literal
import subprocess
import ffmpeg
import os

from tts_webui.utils.outputs.path import get_relative_output_path_ext


def extension__tts_generation_webui():
    return {
        "package_name": "extension_decorator_save_ogg",
        "name": "Decorator Save Ogg",
        "requirements": "git+https://github.com/rsxdalv/extension_decorator_save_ogg@main",
        "description": "Decorator Save Ogg",
        "extension_type": "decorator",
        "extension_class": "outer",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_decorator_save_ogg",
        "extension_website": "https://github.com/rsxdalv/extension_decorator_save_ogg",
        "extension_platform_version": "0.0.1",
    }


def _check_ffmpegg():
    if not hasattr(ffmpeg, "input"):
        raise ImportError(
            """Incorrect ffmpeg version. Please install ffmpeg-python with `pip install ffmpeg-python`"""
        )


def _save(kwargs, result_dict: Dict[str, Any], format: Literal["ogg", "flac"]):
    if kwargs.get("_type", None) == "bark":
        callback_save_generation_bark(
            audio=result_dict["audio_out"],
            full_generation=result_dict["full_generation"],
            filename=get_relative_output_path_ext(result_dict, "." + format),
            metadata=result_dict["metadata"],
            format=format,
        )
        return

    callback_save_generation_musicgen(
        audio=result_dict["audio_out"],
        filename=get_relative_output_path_ext(result_dict, "." + format),
        metadata=result_dict["metadata"],
        format=format,
    )


def decorator_save_ogg(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        _save(kwargs, result_dict, "ogg")

        return result_dict

    return wrapper


def decorator_save_flac(fn):
    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        _save(kwargs, result_dict, "flac")

        return result_dict

    return wrapper


def decorator_save_ogg_generator(fn):
    def wrapper(*args, **kwargs):
        SAVE_EACH = kwargs.get("generator_save_each", False)
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            if SAVE_EACH:
                _save(kwargs, result_dict, "ogg")
            yield result_dict
        if not SAVE_EACH:
            _save(kwargs, result_dict, "ogg")

    return wrapper


def decorator_save_flac_generator(fn):
    def wrapper(*args, **kwargs):
        SAVE_EACH = kwargs.get("generator_save_each", False)
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            if SAVE_EACH:
                _save(kwargs, result_dict, "flac")
            yield result_dict
        if not SAVE_EACH:
            _save(kwargs, result_dict, "flac")

    return wrapper


def callback_save_generation_musicgen(
    audio: tuple[int, np.ndarray],
    filename: str,
    metadata: Dict[str, Any],
    format: Literal["ogg", "flac"],
) -> None:
    _check_ffmpegg()
    SAMPLE_RATE, audio_array = audio
    print("Saving generation to", filename)

    input_data = audio_array.tobytes()
    metadata["text"] = metadata.get("text", "")
    metadata["text"] = _double_escape_quotes(metadata["text"])
    metadata["text"] = _double_escape_newlines(metadata["text"])
    metadata_str = json.dumps(metadata, ensure_ascii=False)

    channels = audio_array.shape[1] if len(audio_array.shape) > 1 else 1
    pipe_input = ffmpeg.input("pipe:", format="f32le", ar=str(SAMPLE_RATE), ac=channels)
    with NamedTemporaryFile("wb", suffix=".ffmetadata.ini", delete=False) as f:
        f.write(f";FFMETADATA1\ncomment={metadata_str}".encode("utf-8"))
        f.flush()
        f.close()
        metadata_input = ffmpeg.input(f.name)

        args = (
            ffmpeg.output(
                pipe_input,
                metadata_input,
                filename,
                format=format,
                map_metadata=f"1",
                loglevel="error",
            )
            .overwrite_output()
            .get_args()
        )

        def find_map_1_index(args: List[str]) -> int:
            return next(
                (
                    i
                    for i in range(len(args))
                    if args[i] == "-map" and args[i + 1] == "1"
                ),
                0,
            )

        def remove_map_1(args: List[str]) -> List[str]:
            index = find_map_1_index(args)
            return args[:index] + args[index + 2 :] if index > 0 else args

        args = remove_map_1(args)

        p = subprocess.Popen(
            ["ffmpeg"] + args, stdin=subprocess.PIPE, stderr=subprocess.PIPE
        )
        output_data = p.communicate(input=input_data)

        p.wait()

        if p.returncode == 0:
            print("Saved generation to", filename)
        else:
            print("Failed to save generation to", filename)
            print("ffmpeg args:", args)
            print(output_data[0])
            # print(output_data[1])
            # print b'ffmpeg
            print("ffmpeg stderr:", output_data[1].decode("utf-8"))

        os.remove(f.name)


def _double_escape_newlines(x: str):
    return x.replace("\n", "\\\n")


def _double_escape_quotes(x: str):
    return x.replace('"', '\\"')


def callback_save_generation_bark(
    audio: tuple[int, np.ndarray],
    full_generation: Any,  # FullGeneration,
    # files: Dict[str, str],
    filename: str,
    metadata: Dict[str, Any],
    format: Literal["ogg", "flac"],
) -> None:
    import base64

    def _ndarray_to_base64(arr):
        # Convert ndarray to bytes
        arr_bytes = arr.tobytes()
        # Encode bytes to base64
        return base64.b64encode(arr_bytes).decode("utf-8")

    def _decode_base64_to_ndarray(base64_string):
        # Decode base64 string to bytes
        decoded_bytes = base64.b64decode(base64_string)
        # Convert bytes to NumPy ndarray
        return np.frombuffer(decoded_bytes, dtype=np.int16)

    def _attach_generation_meta(full_generation, arg1, metadata):
        semantic_prompt: np.ndarray = full_generation[arg1]
        semantic_prompt_base64 = _ndarray_to_base64(semantic_prompt)
        metadata[arg1] = semantic_prompt_base64

    _check_ffmpegg()

    SAMPLE_RATE, audio_array = audio
    print("Saving generation to", filename)

    _attach_generation_meta(full_generation, "semantic_prompt", metadata)
    _attach_generation_meta(full_generation, "coarse_prompt", metadata)
    metadata["text"] = metadata.get("text", "")
    metadata["text"] = _double_escape_quotes(metadata["text"])
    metadata["text"] = _double_escape_newlines(metadata["text"])

    def double_escape_backslash(prompt):
        if prompt is None:
            return None
        return prompt.replace("\\", "\\\\")

    metadata["history_prompt"] = double_escape_backslash(metadata["history_prompt"])
    metadata["history_prompt_npz"] = double_escape_backslash(
        metadata["history_prompt_npz"]
    )
    metadata_str = json.dumps(metadata, ensure_ascii=False)

    pipe_input = ffmpeg.input("pipe:", format="f32le", ar=str(SAMPLE_RATE))
    with NamedTemporaryFile("wb", suffix=".ffmetadata.ini", delete=False) as f:
        f.write(f";FFMETADATA1\ncomment={metadata_str}".encode("utf-8"))
        f.flush()
        f.close()

        metadata_input = ffmpeg.input(f.name)

        args = (
            ffmpeg.output(
                pipe_input,
                metadata_input,
                filename,
                format=format,
                map_metadata=f"1",
                loglevel="error",
            )
            .overwrite_output()
            .get_args()
        )

        def find_map_1_index(args: List[str]) -> int:
            return next(
                (
                    i
                    for i in range(len(args))
                    if args[i] == "-map" and args[i + 1] == "1"
                ),
                0,
            )

        def remove_map_1(args: List[str]) -> List[str]:
            index = find_map_1_index(args)
            return args[:index] + args[index + 2 :] if index > 0 else args

        args = remove_map_1(args)

        p = subprocess.Popen(
            ["ffmpeg"] + args, stdin=subprocess.PIPE, stderr=subprocess.PIPE
        )
        input_data = audio_array.tobytes()
        output_data = p.communicate(input=input_data)

        p.wait()

        if p.returncode == 0:
            print("Saved generation to", filename)
        else:
            print("Failed to save generation to", filename)
            print("ffmpeg args:", args)
            print(output_data[0])
            # print(output_data[1])
            # print b'ffmpeg
            print("ffmpeg stderr:", output_data[1].decode("utf-8"))

        os.remove(f.name)


if __name__ == "__main__":
    wav_input = "./temp/ogg-vs-npz/audio__bark__None__2023-05-29_10-12-46.wav"
    args_output = "./temp/ogg-vs-npz/audio__bark__None__2023-05-29_10-12-46.ogg"

    audio_array = np.load(wav_input)

    metadata = {
        "_version": "0.0.1",
        "_hash_version": "0.0.2",
        "_type": "bark",
        "is_big_semantic_model": False,
        "is_big_coarse_model": False,
        "is_big_fine_model": True,
        "prompt": "♪ これはテストです。",
        "language": None,
        "speaker_id": None,
        "hash": "04d5509a7fd1fabda167e219812ee617",
        "history_prompt": "None",
        "history_prompt_npz": None,
        "history_hash": "6adf97f83acf6453d4a6a4b1070f3754",
        "text_temp": 0.4,
        "waveform_temp": 0.85,
        "date": "2023-06-09_09-40-36",
        "seed": "1542369587",
    }

    callback_save_generation_musicgen(
        audio=(48000, audio_array),
        filename=args_output,
        metadata=metadata,
        format="flac",
    )

    b = ffmpeg.probe(args_output)
    import json

    print(json.dumps(b, indent=4, sort_keys=True))

    x = b["streams"][0]["tags"]["comment"]
    print(x)
    b = json.loads(x)
    print(b)
    print(b["prompt"])
