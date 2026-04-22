import importlib

if __name__ == "__main__":
    import os

    import gradio as gr

    from tts_webui.config.config import config
    from tts_webui.gradio.get_theme import get_theme

    package_name = os.environ.get(
        "TTS_WEBUI_EXTENSION_PACKAGE", "tts_webui_extension.mms.main"
    )
    module = importlib.import_module(f"{package_name}.main")
    main_tab = getattr(module, "extension__tts_generation_webui")

    with gr.Blocks(
        css="""
        main {
            max-width: 100% !important;
            padding: 0 !important;
        }
        """,
        theme=get_theme(config),
    ) as extension_blocks:
        main_tab()

    extension_blocks.launch(share=False, inbrowser=False)
