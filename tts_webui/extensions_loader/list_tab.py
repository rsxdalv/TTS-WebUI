import gradio as gr

from tts_webui.core_bridge import bridge


def extension_list_ui(kind: str):
    """The body of the two list tabs. ``kind`` is "interface" or "decorator".

    Go builds the table and the two dropdowns (they come back as components and
    render where they are created) and runs the installs, streaming the output.
    """
    gr.Markdown("List of all extensions")
    bridge.extcatalog.table(kind)

    with gr.Row():
        with gr.Column():
            gr.Markdown("Install/Uninstall Extensions")
            install_dropdown = bridge.extcatalog.picker(
                kind, "Select Extension to Install"
            )
            install_button = gr.Button("Install extension")
            install_button.click(
                fn=bridge.extcatalog.install,
                inputs=[install_dropdown],
                outputs=[gr.HTML()],
                api_name="install_extension",
            )

        with gr.Column():
            gr.Markdown("Uninstall Extensions")
            uninstall_dropdown = bridge.extcatalog.picker(
                kind, "Select Extension to Uninstall"
            )
            uninstall_button = gr.Button("Uninstall extension")
            uninstall_button.click(
                fn=bridge.extcatalog.uninstall,
                inputs=[uninstall_dropdown],
                outputs=[gr.HTML()],
                api_name="uninstall_extension",
            )
