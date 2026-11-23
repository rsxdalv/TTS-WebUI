"""Log Viewer: view, search and manage the log files in installer_scripts/logs.

Port of tts_webui_extension.log_viewer (MIT, rsxdalv). The file work (listing,
reading, searching, analysing, deleting) is done by the Go core, reached through
tts_webui.core_bridge (bridge.logs); this file only builds the Gradio layout.
"""

import gradio as gr

from tts_webui.core_bridge import bridge

NO_FILE = "No file selected"
NO_ANALYSIS = "Select a log file to see analysis"


def _choices(kind):
    return [(x["name"], x["file"]) for x in bridge.logs.pip_logs(kind)]


def _read(filename, search_term, max_lines):
    if not filename:
        return NO_FILE
    return bridge.logs.read(filename, search_term or "", int(max_lines or 1000))


def _info_and_content(filename, search_term, max_lines):
    if filename:
        return bridge.logs.info(filename), _read(filename, search_term, max_lines)
    return NO_FILE, ""


def _info_and_analysis(filename):
    if filename:
        return bridge.logs.info(filename), bridge.logs.analyze(filename)
    return NO_FILE, NO_ANALYSIS


def _status_visible(status):
    return gr.update(visible=bool(status))


def _raw_log_controls(prefix):
    with gr.Row():
        search = gr.Textbox(
            label="Search", placeholder="Enter text to filter...", scale=3
        )
        max_lines = gr.Number(
            label="Max Lines",
            value=1000,
            minimum=100,
            maximum=10000,
            step=100,
            scale=1,
        )
        view_btn = gr.Button("👁️ View", scale=1)
    content = gr.Textbox(
        label="Log Content",
        lines=25,
        max_lines=25,
        show_copy_button=True,
        interactive=False,
    )
    return search, max_lines, view_btn, content


def _pip_tab(kind):
    """The Pip Install / Pip Uninstall tab (they differ in names and summary)."""
    install = kind == "install"
    with gr.Row():
        with gr.Column(scale=1):
            gr.Markdown("### Installation Logs" if install else "### Uninstallation Logs")
            summary = gr.Markdown(bridge.logs.summary(kind))
            refresh_btn = gr.Button("🔄 Refresh", size="sm")
            dropdown = gr.Dropdown(
                label="Select Installation Log"
                if install
                else "Select Uninstallation Log",
                choices=_choices(kind),
                value=None,
                interactive=True,
            )
            info = gr.Markdown(NO_FILE)

        with gr.Column(scale=3):
            with gr.Tabs():
                with gr.Tab("📊 Analysis"):
                    analysis = gr.Markdown(NO_ANALYSIS)
                with gr.Tab("📄 Raw Log"):
                    search, max_lines, view_btn, content = _raw_log_controls(kind)

    def refresh():
        return (
            gr.update(choices=_choices(kind), value=None),
            bridge.logs.summary(kind),
            NO_FILE,
            NO_ANALYSIS,
        )

    refresh_btn.click(
        fn=refresh, inputs=[], outputs=[dropdown, summary, info, analysis]
    )
    dropdown.change(
        fn=_info_and_analysis, inputs=[dropdown], outputs=[info, analysis]
    )
    view_btn.click(
        fn=_read, inputs=[dropdown, search, max_lines], outputs=[content]
    )
    search.submit(fn=_read, inputs=[dropdown, search, max_lines], outputs=[content])


def _all_logs_tab():
    with gr.Row():
        with gr.Column(scale=1):
            gr.Markdown("### Log Files")
            stats = gr.Markdown(bridge.logs.stats())
            refresh_btn = gr.Button("🔄 Refresh List", size="sm")
            dropdown = gr.Dropdown(
                label="Select Log File",
                choices=bridge.logs.files(),
                value=None,
                interactive=True,
            )
            info = gr.Markdown(NO_FILE)
            with gr.Row():
                delete_btn = gr.Button("🗑️ Delete Selected", size="sm", variant="stop")
                delete_all_btn = gr.Button("🗑️ Delete All", size="sm", variant="stop")

        with gr.Column(scale=3):
            gr.Markdown("### Log Content")
            search, max_lines, view_btn, content = _raw_log_controls("all")
            status = gr.Textbox(label="Status", visible=False)

    def refresh():
        return (
            gr.update(choices=bridge.logs.files(), value=None),
            bridge.logs.stats(),
            NO_FILE,
            "",
        )

    refresh_btn.click(fn=refresh, inputs=[], outputs=[dropdown, stats, info, content])
    dropdown.change(
        fn=_info_and_content,
        inputs=[dropdown, search, max_lines],
        outputs=[info, content],
    )
    view_btn.click(fn=_read, inputs=[dropdown, search, max_lines], outputs=[content])
    search.submit(fn=_read, inputs=[dropdown, search, max_lines], outputs=[content])

    def after_delete(res):
        return (
            res["status"],
            gr.update(choices=res["files"], value=None),
            bridge.logs.stats(),
            NO_FILE,
            "",
        )

    def delete_selected(filename):
        if not filename:
            return after_delete({"status": NO_FILE, "files": bridge.logs.files()})
        return after_delete(bridge.logs.delete(filename))

    def delete_all():
        return after_delete(bridge.logs.delete_all())

    outputs = [status, dropdown, stats, info, content]
    delete_btn.click(fn=delete_selected, inputs=[dropdown], outputs=outputs).then(
        fn=_status_visible, inputs=[status], outputs=[status]
    )
    delete_all_btn.click(fn=delete_all, inputs=[], outputs=outputs).then(
        fn=_status_visible, inputs=[status], outputs=[status]
    )


def log_viewer_ui():
    gr.Markdown(
        """
    # 📋 Log Viewer

    View and manage log files from the TTS Generation WebUI.
    """
    )
    with gr.Tabs():
        with gr.Tab("📦 Pip Install Logs"):
            _pip_tab("install")
        with gr.Tab("🗑️ Pip Uninstall Logs"):
            _pip_tab("uninstall")
        with gr.Tab("📁 All Logs"):
            _all_logs_tab()


def extension__tts_generation_webui():
    log_viewer_ui()

    return {
        "package_name": "extensions.builtin.extension_log_viewer",
        "name": "Log Viewer",
        "requirements": "git+https://github.com/rsxdalv/tts_webui_extension.log_viewer@main",
        "description": "View, search, and manage log files from the TTS Generation WebUI. Browse installation logs, filter by keywords, and clean up old logs.",
        "extension_type": "interface",
        "extension_class": "settings",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts-generation-webui",
        "extension_website": "https://github.com/rsxdalv/tts_webui_extension.log_viewer",
        "extension_platform_version": "0.1.0",
    }


if __name__ == "__main__":
    if "demo" in locals():
        locals()["demo"].close()
    with gr.Blocks() as demo:
        with gr.Tab("Log viewer", id="log_viewer"):
            log_viewer_ui()

    demo.launch(server_port=7772)
