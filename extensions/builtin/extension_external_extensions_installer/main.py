import json
import os
import subprocess
from typing import Any, Dict, List, Tuple

import gradio as gr

from tts_webui.utils.pip_install import pip_install_wrapper
from .messaging_js import messaging_js

EXTERNAL_EXTENSIONS_FILE = "extensions.external.json"
CATALOG_DIR = os.path.join("data", "extensions-catalog")
CATALOG_REPO = "https://github.com/rsxdalv/tts-webui-extension-catalog.git"
CATALOG_JSON_PATH = os.path.join(CATALOG_DIR, "lib", "extensions.json")


def _load_external_extensions() -> Dict[str, Any]:
    try:
        with open(EXTERNAL_EXTENSIONS_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except FileNotFoundError:
        return {"tabs": [], "decorators": []}
    except Exception as e:
        gr.Warning(f"Failed to read {EXTERNAL_EXTENSIONS_FILE}: {e}")
        return {"tabs": [], "decorators": []}


def _save_external_extensions(data: Dict[str, Any]) -> Tuple[bool, str]:
    try:
        with open(EXTERNAL_EXTENSIONS_FILE, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=4, ensure_ascii=False)
        return True, "Saved"
    except Exception as e:
        return False, f"Failed to save: {e}"


REQUIRED_KEYS = {
    "package_name": str,
    "name": str,
    "requirements": str,
    "description": str,
    "extension_type": str,
    "extension_class": str,
    "author": str,
    "extension_author": str,
    "license": str,
    "website": str,
    "extension_website": str,
    "extension_platform_version": str,
}


def _validate_entry(entry: Dict[str, Any]) -> Tuple[bool, str]:
    missing = [k for k in REQUIRED_KEYS if k not in entry]
    if missing:
        return False, f"Missing keys: {', '.join(missing)}"
    bad_types = [k for k, t in REQUIRED_KEYS.items() if not isinstance(entry.get(k), t)]
    if bad_types:
        return False, f"Wrong types for: {', '.join(bad_types)}"
    return True, "OK"


def _parse_json_input(text: str) -> Tuple[List[Dict[str, Any]], str]:
    text = text.strip()
    if not text:
        return [], "No JSON provided"
    try:
        data = json.loads(text)
    except json.JSONDecodeError as e:
        return [], f"JSON parse error: {e}"

    # Accept either a single object or a list of objects
    entries: List[Dict[str, Any]]
    if isinstance(data, dict):
        entries = [data]
    elif isinstance(data, list):
        entries = [d for d in data if isinstance(d, dict)]
    else:
        return [], "Expected a JSON object or an array of objects"

    # Validate entries
    problems = []
    valid_entries: List[Dict[str, Any]] = []
    for i, e in enumerate(entries):
        ok, msg = _validate_entry(e)
        if ok:
            valid_entries.append(e)
        else:
            problems.append(f"Entry {i}: {msg}")

    info = "Parsed entries: " + str(len(valid_entries))
    if problems:
        info += " | Issues: " + " ; ".join(problems)
    return valid_entries, info


def _render_preview(entries: List[Dict[str, Any]]) -> str:
    if not entries:
        return "<i>No valid entries to preview.</i>"
    header = "| Name | Package | Class | Type |\n|---|---|---|---|\n"
    rows = []
    for e in entries:
        rows.append(
            f"| {e['name']} | {e['package_name']} | {e['extension_class']} | {e['extension_type']} |"
        )
    return gr.Markdown(header + "\n".join(rows))


def _sync_catalog_via_git() -> Tuple[str, str]:
    # Try: clone if missing, else pull fast-forward; return output or error.
    try:
        if not os.path.isdir(CATALOG_DIR) or not os.path.isdir(
            os.path.join(CATALOG_DIR, ".git")
        ):
            cmd = ["git", "clone", "--depth=1", CATALOG_REPO, CATALOG_DIR]
        else:
            cmd = ["git", "-C", CATALOG_DIR, "pull", "--ff-only"]
        proc = subprocess.run(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True
        )
        if proc.returncode != 0:
            msg = proc.stderr.strip() or proc.stdout.strip() or "Unknown git error"
            return f"Git failed: {msg}", ""

        # Read catalog JSON from cloned repo
        if not os.path.exists(CATALOG_JSON_PATH):
            return f"Catalog JSON not found at {CATALOG_JSON_PATH}", ""

        with open(CATALOG_JSON_PATH, "r", encoding="utf-8") as f:
            data = json.load(f)

        return "Synced catalog via Git", json.dumps(data, indent=2, ensure_ascii=False)
    except Exception as e:
        return f"Git sync error: {e}", ""


def _add_to_external(entries: List[Dict[str, Any]]) -> Tuple[str, str]:
    if not entries:
        return "Nothing to add.", ""
    data = _load_external_extensions()
    tabs = data.get("tabs", [])

    existing_packages = {t.get("package_name") for t in tabs}
    added = []
    skipped = []
    for e in entries:
        if e.get("package_name") in existing_packages:
            skipped.append(e.get("package_name"))
        else:
            tabs.append(e)
            added.append(e.get("package_name"))
    data["tabs"] = tabs
    ok, msg = _save_external_extensions(data)
    status = (
        ("Saved" if ok else msg)
        + f" | Added: {', '.join(added) if added else 'none'} | Skipped (exists): {', '.join(skipped) if skipped else 'none'}"
    )
    return status, json.dumps(data, indent=2, ensure_ascii=False)


def _install_selected(entries: List[Dict[str, Any]]):
    # Generator to stream console logs using existing wrapper
    if not entries:
        yield "<i>Nothing selected to install.</i>"
        return
    for e in entries:
        req = e.get("requirements")
        name = e.get("name", e.get("package_name", "extension"))
        yield from pip_install_wrapper(req, name)()


EXTENSION_CATALOG_IFRAME_ID = "extension-catalog"


def extension__tts_generation_webui():
    gr.Markdown(
        """
    Paste one or more extension JSON objects below. We'll validate, preview, and let you add them to `extensions.external.json` and optionally install their requirements right away.
    """
    )

    with gr.Accordion("Browse Extension Catalog", open=True):
        gr.HTML(
            f"""<iframe id={EXTENSION_CATALOG_IFRAME_ID} 
                 width="100%"
                 height="800px"
                 src="https://rsxdalv.github.io/tts-webui-extension-catalog/?browse=true&iframe=true"
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

    with gr.Accordion("Install Console", open=True):
        console_html = gr.HTML()

    with gr.Accordion("Current extensions.external.json", open=False):
        current_json = gr.Code(language="json")

    with gr.Accordion("Catalog JSON (data/extensions-catalog)", open=False):
        sync_btn = gr.Button("Sync Catalog via Git", variant="secondary")

        latest_json = gr.Code(language="json")

    def _on_parse(text: str):
        entries, info = _parse_json_input(text)
        return entries, _render_preview(entries), info

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

    json_input_automatic.change(
        fn=_on_parse,
        inputs=[json_input_automatic],
        outputs=[parsed_state, preview_md, parse_info],
    ).then(
        fn=_add_to_external,
        inputs=[parsed_state],
        outputs=[parse_info, current_json],
    ).then(
        fn=_install_selected,
        inputs=[parsed_state],
        outputs=[console_html],
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

    def _on_sync():
        status, content = _sync_catalog_via_git()
        return status, content

    sync_btn.click(
        fn=_on_sync,
        inputs=[],
        outputs=[parse_info, latest_json],
    )

    return {
        "package_name": "extensions.builtin.extension_external_extensions_installer",
        "name": "External Extensions Installer",
        "requirements": "builtin",
        "description": "Add external extension entries via JSON and install them without restarts. Sync and apply the public extensions catalog via Git.",
        "extension_type": "interface",
        "extension_class": "settings",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts-generation-webui",
        "extension_website": "",
        "extension_platform_version": "0.0.1",
    }


if __name__ == "__main__":
    if "demo" in locals():
        locals()["demo"].close()
    with gr.Blocks() as demo:
        with gr.Tab("External Extensions Installer"):
            extension__tts_generation_webui()

    demo.launch()
