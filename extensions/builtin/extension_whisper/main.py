import gradio as gr
import torch
import os

from typing import TYPE_CHECKING

from tts_webui.utils.manage_model_state import manage_model_state
from tts_webui.utils.list_dir_models import unload_model_button

if TYPE_CHECKING:
    from transformers import Pipeline


def extension__tts_generation_webui():
    transcribe_ui()
    return {
        "package_name": "extension_whisper",
        "name": "Whisper",
        "requirements": "git+https://github.com/rsxdalv/extension_whisper@main",
        "description": "Whisper allows transcribing audio files.",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_whisper",
        "extension_website": "https://github.com/rsxdalv/extension_whisper",
        "extension_platform_version": "0.0.1",
    }


@manage_model_state("whisper")
def get_model(
    model_name="openai/whisper-large-v3",
    torch_dtype=torch.float16,
    device="cuda:0",
    compile=False,
):
    from transformers import AutoModelForSpeechSeq2Seq
    from transformers import AutoProcessor

    model = AutoModelForSpeechSeq2Seq.from_pretrained(
        model_name, torch_dtype=torch_dtype, low_cpu_mem_usage=True
    ).to(device)
    if compile:
        model.generation_config.cache_implementation = "static"
        model.generation_config.max_new_tokens = 256
        model.forward = torch.compile(
            model.forward, mode="reduce-overhead", fullgraph=True
        )

    processor = AutoProcessor.from_pretrained(model_name)

    return model, processor


local_dir = os.path.join("data", "models", "whisper")
local_cache_dir = os.path.join(local_dir, "cache")


@manage_model_state("whisper-pipe")
def get_pipe(model_name, device="cuda:0") -> "Pipeline":
    from transformers import pipeline

    torch_dtype = torch.float16

    model, processor = get_model(
        # model_name, torch_dtype=torch.float16, device=device, compile=False
        model_name,
        torch_dtype=torch_dtype,
        device=device,
        compile=False,
    )
    return pipeline(
        "automatic-speech-recognition",
        model=model,
        tokenizer=processor.tokenizer,
        feature_extractor=processor.feature_extractor,
        # chunk_length_s=30,
        # batch_size=16,  # batch size for inference - set based on your device
        torch_dtype=torch.float16,
        model_kwargs={"cache_dir": local_cache_dir},
        device=device,
    )


def transcribe(inputs, model_name="openai/whisper-large-v3"):
    if inputs is None:
        raise gr.Error(
            "No audio file submitted! Please record an audio before submitting your request."
        )

    pipe = get_pipe(model_name)

    result = pipe(
        inputs,
        generate_kwargs=(
            {"task": "transcribe"} if model_name == "openai/whisper-large-v3" else {}
        ),
        return_timestamps=True,
    )
    return result["text"]


def transcribe_ui():
    gr.Markdown(
        "Transcribe long-form microphone or audio inputs with the click of a button! Demo uses the"
        " checkpoint [openai/whisper-large-v3](https://huggingface.co/openai/whisper-large-v3) and 🤗 Transformers to transcribe audio files"
        " of arbitrary length."
    )

    with gr.Row():
        with gr.Column():
            audio = gr.Audio(label="Audio", type="filepath", sources="upload")
            model_dropdown = gr.Dropdown(
                choices=[
                    "openai/whisper-tiny.en",
                    "openai/whisper-small.en",
                    "openai/whisper-medium.en",
                    "openai/whisper-large-v3",
                ],
                label="Model",
                value="openai/whisper-large-v3",
            )
        with gr.Column():
            text = gr.Textbox(label="Transcription", interactive=False)

    with gr.Row():
        unload_model_button("whisper-pipe")
        unload_model_button("whisper")

        transcribe_button = gr.Button("Transcribe", variant="primary")

    transcribe_button.click(
        fn=transcribe,
        inputs=[audio, model_dropdown],
        outputs=[text],
        api_name="whisper_transcribe",
    )


if __name__ == "__main__":
    if "demo" in locals():
        locals()["demo"].close()

    with gr.Blocks() as demo:
        with gr.Tab("Whisper"):
            transcribe_ui()

    demo.queue().launch()
