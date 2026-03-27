import json
import os
import time
import urllib.request
import urllib.error
from datetime import datetime

import gradio as gr


MINIMAX_API_URL = "https://api.minimax.io/v1/t2a_v2"

MINIMAX_MODELS = [
    "speech-2.8-hd",
    "speech-2.8-turbo",
]

MINIMAX_VOICES = [
    "English_Graceful_Lady",
    "English_Insightful_Speaker",
    "English_radiant_girl",
    "English_Persuasive_Man",
    "English_Lucky_Robot",
    "Wise_Woman",
    "cute_boy",
    "lovely_girl",
    "Friendly_Person",
    "Inspirational_girl",
    "Deep_Voice_Man",
    "sweet_girl",
]


def _get_api_key(api_key_input: str) -> str:
    if api_key_input and api_key_input.strip():
        return api_key_input.strip()
    return os.environ.get("MINIMAX_API_KEY", "")


def _save_audio(audio_hex: str, text: str, model: str, voice_id: str) -> tuple:
    audio_bytes = bytes.fromhex(audio_hex)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    safe_text = "".join(c if c.isalnum() or c in " _-" else "" for c in text[:50])
    safe_text = safe_text.strip().replace(" ", "_") or "minimax_tts"
    folder_name = f"{timestamp}_{safe_text}"

    output_dir = os.path.join("outputs", folder_name)
    os.makedirs(output_dir, exist_ok=True)

    audio_path = os.path.join(output_dir, f"{safe_text}.mp3")
    with open(audio_path, "wb") as f:
        f.write(audio_bytes)

    metadata = {
        "text": text,
        "model": model,
        "voice_id": voice_id,
        "provider": "minimax",
        "_type": "minimax_cloud_tts",
        "_version": "0.1.0",
    }

    metadata_path = audio_path.replace(".mp3", ".json")
    with open(metadata_path, "w") as f:
        json.dump(metadata, f, indent=2)

    return audio_path, metadata, output_dir


def generate_minimax_tts(
    text: str,
    model: str,
    voice_id: str,
    api_key: str,
    **kwargs,
) -> tuple:
    key = _get_api_key(api_key)
    if not key:
        raise gr.Error(
            "MiniMax API key is required. Set MINIMAX_API_KEY environment variable "
            "or enter it in the API Key field."
        )

    if not text or not text.strip():
        raise gr.Error("Please enter text to synthesize.")

    payload = {
        "model": model,
        "text": text.strip(),
        "voice_setting": {
            "voice_id": voice_id,
        },
        "audio_setting": {
            "format": "mp3",
        },
    }

    headers = {
        "Authorization": f"Bearer {key}",
        "Content-Type": "application/json",
    }

    req = urllib.request.Request(
        MINIMAX_API_URL,
        data=json.dumps(payload).encode("utf-8"),
        headers=headers,
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            result = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        raise gr.Error(f"MiniMax API error ({e.code}): {body}")
    except urllib.error.URLError as e:
        raise gr.Error(f"Network error: {e.reason}")

    if "base_resp" in result and result["base_resp"].get("status_code", 0) != 0:
        msg = result["base_resp"].get("status_msg", "Unknown error")
        raise gr.Error(f"MiniMax API error: {msg}")

    audio_hex = result.get("data", {}).get("audio")
    if not audio_hex:
        raise gr.Error(
            "MiniMax API returned no audio data. "
            "Check your API key and request parameters."
        )

    audio_path, metadata, folder_root = _save_audio(audio_hex, text, model, voice_id)
    return audio_path, metadata, folder_root


def minimax_cloud_tts_tab():
    with gr.Column():
        gr.Markdown(
            "Generate speech using [MiniMax](https://www.minimaxi.com) Cloud TTS API. "
            "Requires a MiniMax API key (set `MINIMAX_API_KEY` env var or enter below)."
        )

        with gr.Row():
            with gr.Column(scale=2):
                text_input = gr.Textbox(
                    label="Text",
                    placeholder="Enter text to synthesize...",
                    lines=4,
                )

            with gr.Column(scale=1):
                model_dropdown = gr.Dropdown(
                    label="Model",
                    choices=MINIMAX_MODELS,
                    value="speech-2.8-hd",
                )
                voice_dropdown = gr.Dropdown(
                    label="Voice",
                    choices=MINIMAX_VOICES,
                    value="English_Graceful_Lady",
                )
                api_key_input = gr.Textbox(
                    label="API Key (optional if MINIMAX_API_KEY is set)",
                    placeholder="Enter MiniMax API key...",
                    type="password",
                )

        generate_btn = gr.Button("Generate", variant="primary")
        audio_output = gr.Audio(label="Generated Audio", type="filepath")
        metadata_output = gr.JSON(label="Metadata")

        generate_btn.click(
            fn=generate_minimax_tts,
            inputs=[text_input, model_dropdown, voice_dropdown, api_key_input],
            outputs=[audio_output, metadata_output, gr.Textbox(visible=False)],
            api_name="minimax_cloud_tts",
        )


def extension__tts_generation_webui():
    minimax_cloud_tts_tab()
    return {
        "package_name": "extensions.builtin.extension_minimax_cloud_tts",
        "name": "MiniMax Cloud TTS",
        "requirements": "",
        "description": "Cloud-based text-to-speech using MiniMax API with speech-2.8-hd and speech-2.8-turbo models",
        "extension_type": "interface",
        "extension_class": "text-to-speech",
        "author": "MiniMax",
        "extension_author": "octo-patch",
        "license": "MIT",
        "website": "https://www.minimaxi.com",
        "extension_website": "https://github.com/rsxdalv/TTS-WebUI",
        "extension_platform_version": "0.0.1",
    }
