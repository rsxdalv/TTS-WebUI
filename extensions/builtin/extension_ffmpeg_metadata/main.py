import ffmpeg
import json
import os
import gradio as gr


def extension__tts_generation_webui():
    ffmpeg_metadata_ui()
    return {
        "package_name": "extension_ffmpeg_metadata",
        "name": "FFMPEG Metadata",
        "requirements": "git+https://github.com/rsxdalv/extension_ffmpeg_metadata@main",
        "description": "FFMPEG Metadata allows loading metadata from audio files.",
        "extension_type": "interface",
        "extension_class": "outputs",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_ffmpeg_metadata",
        "extension_website": "https://github.com/rsxdalv/extension_ffmpeg_metadata",
        "extension_platform_version": "0.0.1",
    }


def load_ffmpeg_metadata(filename: str):
    if not hasattr(ffmpeg, "probe"):
        raise ImportError(
            """Incorrect ffmpeg version. Please install ffmpeg-python with `pip install ffmpeg-python`"""
        )
    if not filename or not os.path.exists(filename):
        return None
    ffmpeg_output = ffmpeg.probe(filename)
    if filename.endswith(".ogg") or ffmpeg_output["format"]["format_name"] == "ogg":
        return json.loads(ffmpeg_output["streams"][0]["tags"]["comment"])
    if filename.endswith(".flac") or ffmpeg_output["format"]["format_name"] == "flac":
        return json.loads(ffmpeg_output["format"]["tags"]["comment"])
    print("Unknown file type:", filename)
    print(json.dumps(ffmpeg_output, indent=4, sort_keys=True))
    return json.loads(json.dumps(ffmpeg_output, indent=4, sort_keys=True))


def ffmpeg_metadata_ui():
    with gr.Column():
        input_file = gr.File(label="Input File")
        audio_preview = gr.Audio(label="Audio Preview")
        metadata = gr.JSON(label="Metadata")
    input_file.change(
        fn=load_ffmpeg_metadata,
        inputs=input_file,
        outputs=metadata,
        api_name="load_ffmpeg_metadata",
    ).then(
        fn=lambda x: gr.Audio(value=x),
        inputs=input_file,
        outputs=audio_preview,
    )


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()
    with gr.Blocks() as demo:
        with gr.Tab("FFMPEG Metadata"):
            ffmpeg_metadata_ui()
    demo.launch(
        server_port=7770,
    )
