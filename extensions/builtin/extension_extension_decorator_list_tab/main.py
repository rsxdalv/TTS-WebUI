import gradio as gr

from tts_webui.extensions_loader.decorator_extensions import (
    extension_decorator_list_tab,
)


def extension__tts_generation_webui():
    extension_decorator_list_tab()
    return {
        "package_name": "extension_extension_decorator_list_tab",
        "name": "Decorator Extensions List",
        "requirements": "git+https://github.com/rsxdalv/tts_webui_extension.extension_decorator_list_tab@main",
        "description": "Decorator Extensions List shows the list of decorator extensions in the web UI",
        "extension_type": "interface",
        "extension_class": "settings",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts_webui_extension.extension_decorator_list_tab",
        "extension_website": "https://github.com/rsxdalv/tts_webui_extension.extension_decorator_list_tab",
        "extension_platform_version": "0.0.1",
    }


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()  # type: ignore
    with gr.Blocks() as demo:
        with gr.Tab("Extensions List"):
            extension_decorator_list_tab()

    demo.launch()
