import gradio as gr

from tts_webui.extensions_loader import handle_extension_class


def all_tabs(config):
    for name, extension_class in [
        ("💬 Text-to-Speech", "text-to-speech"),
        ("🎼 Audio/Music Generation", "audio-music-generation"),
        ("🎙️ Audio Conversion", "audio-conversion"),
        ("🤖 Conversational AI", "conversational-ai"),
        ("📁 Outputs", "outputs"),
        ("🧩 Extensions", "extensions"),
        ("🔧 Tools", "tools"),
        ("⚙️ Settings", "settings"),
        ("📚 Tutorials", "tutorials"),
    ]:
        with gr.Tab(name), gr.Tabs():
            handle_extension_class(extension_class, config)
