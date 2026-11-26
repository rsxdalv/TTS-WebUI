import gradio as gr

from tts_webui.core_bridge import bridge
from .messaging_js import messaging_js

EXTENSION_CATALOG_IFRAME_ID = "extension-catalog"


def _on_parse(text: str):
    # Go validates the pasted JSON and builds the preview table.
    res = bridge.extcatalog.parse(text)
    return res["entries"], res["preview"], res["info"]


def _add_to_external(entries):
    # Go is the only writer of extensions.external.json.
    res = bridge.extcatalog.add(entries)
    return res["status"], res["json"]


def _install_selected(entries):
    if not entries:
        yield "<i>Nothing selected to install.</i>"
        return
    for e in entries:
        # Go installs the entry it was given (its own requirements and proxy:
        # "native" means the venv path), not a package it looks up by name.
        yield from bridge.extcatalog.install_entry(e)


def ui():
    gr.HTML(
        f"""<iframe id={EXTENSION_CATALOG_IFRAME_ID}
                width="100%"
                height="750px"
                src="https://rsxdalv.github.io/tts-webui-extension-catalog/?iframe=true"
                title="TTS WebUI Extension Catalog"
                frameborder="0"
        ></iframe>
        <div id="json-container" hidden></div>
        """
    )

    init = gr.Timer(0.5)
    init.tick(
        fn=lambda: gr.Timer(active=False),
        inputs=[],
        outputs=[init],
        js=messaging_js,
    )

    with gr.Accordion("Install Console", open=True):
        console_html = gr.HTML()

    with gr.Accordion("Debug Info", open=False):
        with gr.Row():
            json_input = gr.Textbox(
                label="Extension JSON",
                lines=16,
                placeholder="Paste JSON object or array of objects here",
            )
            json_input_automatic = gr.Textbox(visible=False)
        with gr.Row():
            parse_btn = gr.Button("Parse JSON", variant="primary")
            add_btn = gr.Button("Add to external list", variant="secondary")
            install_btn = gr.Button("Install selected", variant="primary")

        parsed_state = gr.State([])  # holds last parsed valid entries

        with gr.Accordion("Preview", open=True):
            preview_md = gr.Markdown()
            parse_info = gr.HTML()

        with gr.Accordion("Current extensions.external.json", open=False):
            current_json = gr.Code(language="json")

        gr.Button("", elem_id="receive_extension_button").click(
            fn=None,
            inputs=[],
            outputs=[json_input, json_input_automatic],
            js="""
                () => {
                    json_value = document.getElementById('json-container').innerText;
                    return [json_value, json_value];
                }
            """,
        )

        # Parse and preview only. Adding to extensions.external.json and running
        # the installer both execute code from the received JSON, so they stay
        # behind the explicit "Add to external list" / "Install selected"
        # buttons rather than firing automatically on a received message.
        json_input_automatic.change(
            fn=_on_parse,
            inputs=[json_input_automatic],
            outputs=[parsed_state, preview_md, parse_info],
        )

        parse_btn.click(
            fn=_on_parse,
            inputs=[json_input],
            outputs=[parsed_state, preview_md, parse_info],
        )

        add_btn.click(
            fn=_add_to_external,
            inputs=[parsed_state],
            outputs=[parse_info, current_json],
        )

        install_btn.click(
            fn=_install_selected,
            inputs=[parsed_state],
            outputs=[console_html],
        )


def extension__tts_generation_webui():
    ui()

    return {
        "package_name": "extensions.builtin.extension_extension_catalog",
        "name": "Extension Catalog",
        "requirements": "builtin",
        "description": "Browse and import extensions from the online catalog",
        "extension_type": "interface",
        "extension_class": "extensions",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts-webui-extension-catalog",
        "extension_website": "",
        "extension_platform_version": "0.0.1",
    }


if __name__ == "__main__":
    if "demo" in locals():
        locals()["demo"].close()
    with gr.Blocks() as demo:
        with gr.Tab("Catalog"):
            extension__tts_generation_webui()

    demo.launch()
