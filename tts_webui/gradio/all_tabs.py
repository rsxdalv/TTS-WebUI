import gradio as gr

from tts_webui.extensions_loader.interface_extensions import handle_extension_class


def all_tabs(config):
    for name, extension_class in [
        ("🧩 Extensions", "extensions"),
        ("💬 Text-to-Speech", "text-to-speech"),
        ("🎼 Audio/Music Generation", "audio-music-generation"),
        ("🎙️ Audio Conversion", "audio-conversion"),
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
