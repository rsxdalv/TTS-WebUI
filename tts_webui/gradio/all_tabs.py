import gradio as gr

from tts_webui.extensions_loader.interface_extensions import (
    handle_extension_class,
)


def prestart_openai_api_server():
    try:
        import importlib

        module = importlib.import_module("tts_webui_extension.openai_tts_api.main")
        fn = getattr(module, "start_server__tts_generation_webui", None)
        if fn is not None:
            print("[prestart] Starting OpenAI TTS API server")
            fn()
    except Exception as e:
        print(f"[prestart] Failed to start OpenAI TTS API server: {e}")


def all_tabs(config):
    prestart_openai_api_server()
    for name, extension_class in [
        ("🧩 Extensions", "extensions"),
        ("💬 Text-to-Speech", "text-to-speech"),
        ("🎤 Speech-to-Text", "speech-to-text"),
        ("🎼 Audio & Music", "audio-music-generation"),
        ("🎙️ Transform", "audio-conversion"),
        ("🤖 Conversational AI", "conversational-ai"),
        ("📁 Outputs", "outputs"),
        ("🔧 Tools", "tools"),
        ("⚙️ Settings", "settings"),
        ("📚 Tutorials", "tutorials"),
    ]:
        with gr.Tab(name), gr.Tabs():
            if not extension_class == "extensions":
                with gr.Tab("Main"):
                    gr.Markdown(f"## {name}")
                    gr.Markdown("Select a tab to start.")
            handle_extension_class(extension_class, config)
