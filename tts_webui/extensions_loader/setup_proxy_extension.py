import gradio as gr

from tts_webui.config.config_utils import get_config_value
from tts_webui.core_bridge import CoreBridgeError, bridge
from tts_webui.gradio_proxy_tree.main import GRADIO_TREE_URL


def get_openai_api_host():
    port = get_config_value("extension_openai_tts_api", "port", 7778)
    return f"http://localhost:{port}"


def setup_proxy_extension(
    package_name,
    title_name,
    tab,
    autostart=False,
    autoload=True,
    install_trigger=None,
    is_installed=False,
):
    extension_ran = False
    iframe = f'<iframe src="{GRADIO_TREE_URL}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>'

    run_guard = gr.State(False)
    first_load_guard = gr.State(False)

    with gr.Row():
        shutdown_btn = gr.Button(
            "Shutdown Extension", variant="stop", interactive=False
        )
        start_btn = gr.Button("Start Extension", variant="primary", interactive=True)

    def shutdown_extension():
        try:
            nonlocal extension_ran
            bridge.extensions.stop(package_name)
            gr.Info("Extension stopped")
            print(f"Stopped the {title_name} extension")
            extension_ran = False
            return (
                gr.HTML("Extension stopped."),
                gr.Button(interactive=True),
                gr.Button(interactive=False),
            )
        except Exception as e:
            gr.Error(f"Error stopping extension: {e}")
            return gr.skip(), gr.skip(), gr.skip()

    page = gr.HTML(
        "Please install extension"
        if not is_installed
        else "Loading Extension, please wait..."
        if autoload
        else "Press 'Start Extension'"
    )

    def run_extension():
        nonlocal extension_ran

        yield gr.HTML("Starting extension, please wait..."), gr.skip(), gr.skip()

        print(f"\nStarting {title_name} extension in a separate process...")
        try:
            started = bridge.extensions.start(package_name, get_openai_api_host())
            extension_ran = True
            print(f"{title_name} extension runs on port {started['port']}/{package_name}")
            bridge.extensions.wait_ready(package_name)
        except CoreBridgeError as e:
            extension_ran = False
            gr.Warning(f"Could not start {title_name}: {e}")
            yield (
                gr.HTML(f"Could not start the extension: {e}"),
                gr.Button(interactive=True),
                gr.Button(interactive=False),
            )
            return
        yield (
            gr.HTML(iframe, padding=False),
            gr.Button(interactive=False),
            gr.Button(interactive=True),
        )

    def run_extension_guard(first_load_guard_value):
        if extension_ran:
            if first_load_guard_value:
                return gr.skip(), gr.skip()
            else:
                return gr.skip(), gr.State(True)
        else:
            return gr.State(True), gr.skip()

    first_load_guard.change(fn=lambda: gr.HTML(iframe, padding=False), outputs=page)
    shutdown_btn.click(fn=shutdown_extension, outputs=[page, start_btn, shutdown_btn])
    start_btn.click(
        fn=run_extension_guard,
        inputs=[first_load_guard],
        outputs=[run_guard, first_load_guard],
    )
    if autoload:
        tab.select(
            fn=run_extension_guard,
            inputs=[first_load_guard],
            outputs=[run_guard, first_load_guard],
        )
    run_guard.change(fn=run_extension, outputs=[page, start_btn, shutdown_btn])

    if autostart:
        gr.on(triggers=None, fn=lambda: None).then(
            run_extension_guard,
            inputs=[first_load_guard],
            outputs=[run_guard, first_load_guard],
        )

    gr.on(triggers=[install_trigger.change], fn=lambda: None).then(
        run_extension_guard,
        inputs=[first_load_guard],
        outputs=[run_guard, first_load_guard],
    )
