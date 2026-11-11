"""
Proxy harness entry point for TTS WebUI extension sub-processes.

Run as:
    python -m tts_webui_sdk.proxy_harness

Environment variables:
    TTS_WEBUI_EXTENSION_PACKAGE   importable package name of the extension
    GRADIO_SERVER_PORT            port to run the Gradio block on
    GRADIO_ROOT_PATH              URL root path (e.g. /my_extension)
    OPENAI_PROXY_HOST             optional OpenAI-compatible proxy host
"""

import importlib
import os

if __name__ == "__main__":
    import gradio as gr

    package_name = os.environ.get(
        "TTS_WEBUI_EXTENSION_PACKAGE", "tts_webui_extension.mms.main"
    )
    module = importlib.import_module(f"{package_name}.main")
    main_tab = getattr(module, "extension__tts_generation_webui")

    with gr.Blocks(
        css="main { max-width: 100% !important; padding: 0 !important; }",
    ) as blocks:
        main_tab()

    blocks.launch(share=False, inbrowser=False)
