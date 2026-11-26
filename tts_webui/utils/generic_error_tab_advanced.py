import traceback

import gradio as gr

from tts_webui.core_bridge import bridge


def generic_error_tab_advanced(
    e: Exception, name="", requirements=None, package_name=None
):
    with gr.Tab(name + " (!)"):
        # The details go to the console only. Rendering the exception and
        # stack trace in the UI exposed absolute paths, the OS username and the
        # installed package layout to anyone who could reach the server.
        gr.Markdown(f"Failed to load {name} tab. Please check your configuration.")
        gr.Markdown("See the server console for the error and stack trace.")
        print(f"Failed to load {name} tab. Please check your configuration.")
        print(f"Error: {e}")
        print(f"Stacktrace: {traceback.format_exc()}")

        if requirements and package_name:
            gr.Markdown(f"Please install the {requirements} file")
            gr.Markdown("Please check the console for more information")
            install_btn = gr.Button(f"Install {name}")
            gr.Markdown("Installation console:")
            console_text = gr.HTML()

            def install():
                yield from bridge.extcatalog.install(package_name)

            install_btn.click(install, outputs=[console_text])
