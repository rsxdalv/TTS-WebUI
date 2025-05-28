import gradio as gr
import os


def extension__tts_generation_webui():
    model_download_ui()
    return {
        "package_name": "extension_model_downloader",
        "name": "Model Downloader",
        "requirements": "git+https://github.com/rsxdalv/extension_model_downloader@main",
        "description": "Model Downloader allows downloading models from the Huggingface model hub.",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_model_downloader",
        "extension_website": "https://github.com/rsxdalv/extension_model_downloader",
        "extension_platform_version": "0.0.1",
    }


_model_base_dir = os.path.join("data", "models")


def download_pretrained_model(model_type: str, name: str, token: str):
    from huggingface_hub import snapshot_download

    local_dir = os.path.join(
        _model_base_dir,
        model_type,
        name.replace("/", "__").replace(":", "_").replace(".", "_"),
    )

    model_config_path = snapshot_download(
        name,
        repo_type="model",
        local_dir=local_dir,
        local_dir_use_symlinks=False,
        token=token,
    )

    print(model_config_path)

    return "ok"


def model_download_ui():
    gr.Markdown(
        "Models can be found on the [HuggingFace model hub](https://huggingface.co/models?search=whisper)."
    )
    model_type = gr.Textbox(
        label="Model type, e.g. magnet, maha-tts, musicgen_audiogen, parler_tts, rvc, stable-audio, tortoise, vall-e-x, whisper, xtts",
        value="",
    )
    pretrained_name_text = gr.Textbox(
        label="HuggingFace repo name, e.g. openai/whisper-small",
        value="",
    )
    token_text = gr.Textbox(
        label="HuggingFace Token (Optional, but needed for some non-public models)",
        placeholder="hf_nFjKuKLJF...",
        value="",
    )
    download_btn = gr.Button("Download")
    download_btn.click(
        download_pretrained_model,
        inputs=[model_type, pretrained_name_text, token_text],
        outputs=[pretrained_name_text],
        api_name="model_download",
    )


if __name__ == "__main__":
    if "demo" in locals():
        locals()["demo"].close()

    with gr.Blocks() as demo:
        model_download_ui()

    demo.queue().launch()
