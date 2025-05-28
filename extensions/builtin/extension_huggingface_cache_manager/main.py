from typing import Optional
import gradio as gr


def extension__tts_generation_webui():
    scan_cache_ui()
    return {
        "package_name": "extension_huggingface_cache_manager",
        "name": "Huggingface Cache Manager",
        "requirements": "git+https://github.com/rsxdalv/extension_huggingface_cache_manager@main",
        "description": "Huggingface Cache Manager allows managing the Huggingface cache.",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_huggingface_cache_manager",
        "extension_website": "https://github.com/rsxdalv/extension_huggingface_cache_manager",
        "extension_platform_version": "0.0.1",
    }


from huggingface_hub import scan_cache_dir, HFCacheInfo
from extensions.builtin.extension_huggingface_cache_manager.scan_cache import (
    get_headers_quiet,
    get_rows_quiet,
    get_headers_verbose,
    get_rows_verbose,
    get_headers_json,
    get_rows_json,
    render_as_markdown,
)

hf_cache_info: Optional[HFCacheInfo] = None

def scan_cache():
    global hf_cache_info
    hf_cache_info = scan_cache_dir()
    table = render_as_markdown(
        # get_rows_quiet(hf_cache_info), get_headers_quiet()
        get_rows_verbose(hf_cache_info), get_headers_verbose()
    )
    return table

def scan_cache_json():
    global hf_cache_info
    hf_cache_info = scan_cache_dir()
    import json
    headers = get_headers_json()
    data = {
        "headers": headers,
        "rows": [
            {
                header: row[i]
                for i, header in enumerate(headers)
            }
            for row in get_rows_json(hf_cache_info)
        ]
    }
    return json.dumps(data)

def delete_revisions(revision_id):
    global hf_cache_info
    if hf_cache_info is None:
        hf_cache_info = scan_cache_dir()
    strategy = hf_cache_info.delete_revisions(revision_id)
    strategy.execute()


def scan_cache_ui():
    gr.Markdown("Scan the Huggingface cache directory and print the results.")
    scan_cache_button = gr.Button("Scan cache", variant="primary")
    cache_table = gr.Markdown("Press Scan cache to load the list")
    scan_cache_button.click(
        fn=scan_cache,
        outputs=[cache_table],
        api_name="scan_huggingface_cache",
    )

    scan_cache_json_api = gr.JSON(visible=False)
    scan_cache_button_api = gr.Button("API_SCAN_CACHE", visible=False)
    scan_cache_button_api.click(
        fn=scan_cache_json,
        outputs=[scan_cache_json_api],
        api_name="scan_huggingface_cache_api",
    )

    gr.Markdown("Delete revisions")

    delete_revision_id = gr.Dropdown(
        label="Revision ID",
        choices=[""],
        value="",
        show_label=True,
        interactive=True,
    )

    refresh_revision_id_button = gr.Button("Refresh", variant="secondary")
    def refresh_revision_id_button_fn():
        global hf_cache_info
        if hf_cache_info is None:
            hf_cache_info = scan_cache_dir()
        revision_ids = [
            revision.commit_hash
            for repo in hf_cache_info.repos
            for revision in repo.revisions
        ]
        return gr.Dropdown(choices=revision_ids)
        

    refresh_revision_id_button.click(
        fn=refresh_revision_id_button_fn,
        outputs=[delete_revision_id],
        api_name="refresh_huggingface_cache_revisions",
    )

    delete_button = gr.Button("Delete", variant="stop")
    delete_button.click(
        fn=delete_revisions,
        # inputs=[cache_table],
        inputs=[delete_revision_id],
        api_name="delete_huggingface_cache_revisions",
    )


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()

    with gr.Blocks() as demo:
        with gr.Tab("Scan Cache"):
            scan_cache_ui()

    demo.queue().launch(
        server_port=7770,
    )
