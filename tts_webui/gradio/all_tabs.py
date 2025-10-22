import gradio as gr

from tts_webui.extensions_loader import handle_extension_class
from tts_webui.extensions_loader.LoadingIndicator import LoadingIndicator
from tts_webui.utils.generic_error_tab_advanced import generic_error_tab_advanced


def run_tab(module_name, function_name, name, requirements=None):
    import importlib

    with LoadingIndicator(name):
        try:
            module = importlib.import_module(module_name)
            func = getattr(module, function_name)
            func()
        except Exception as e:
            generic_error_tab_advanced(e, name=name, requirements=requirements)


def load_tabs(list_of_tabs):
    for tab in list_of_tabs:
        module_name, function_name, name = tab[:3]
        requirements = tab[3] if len(tab) > 3 else None
        run_tab(module_name, function_name, name, requirements)


def all_tabs(config):
    with gr.Tab("💬 Text-to-Speech"), gr.Tabs():
        handle_extension_class("text-to-speech", config)
    with gr.Tab("🎼 Audio/Music Generation"), gr.Tabs():
        handle_extension_class("audio-music-generation", config)
    with gr.Tab("🎙️ Audio Conversion"), gr.Tabs():
        handle_extension_class("audio-conversion", config)
    with gr.Tab("🤖 Conversational AI"), gr.Tabs():
        handle_extension_class("conversational-ai", config)
    with gr.Tab("📁 Outputs"), gr.Tabs():
        handle_extension_class("outputs", config)
    with gr.Tab("🔧 Tools"), gr.Tabs():
        handle_extension_class("tools", config)
    with gr.Tab("⚙️ Settings"), gr.Tabs():
        settings_tabs = [
            ("tts_webui.extensions_loader", "extension_list_tab", "Extensions List"),
            (
                "tts_webui.extensions_loader",
                "extension_decorator_list_tab",
                "Extension Decorators List",
            ),
            # ("tts_webui.logs_tab", "logs_tab", "Logs"),
            # ("tts_webui.about_tab", "about_tab", "About"),
        ]
        load_tabs(settings_tabs)

        handle_extension_class("settings", config)
    with gr.Tab("📚 Tutorials"), gr.Tabs():
        handle_extension_class("tutorials", config)
