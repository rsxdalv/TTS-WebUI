import functools
import os
import subprocess
import sys

import gradio as gr

from tts_webui.gradio_proxy_tree.main import (
    GRADIO_TREE_HOSTNAME,
    GRADIO_TREE_PORT,
    add_extension_route,
)

PORT_ROUTER = 27770


def wait_for_server(port, timeout=30):
    import socket
    import time

    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(1)
            result = sock.connect_ex(("127.0.0.1", port))
            sock.close()
            if result == 0:
                return True
        except Exception:
            pass
        time.sleep(0.25)
    return False


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
    global PORT_ROUTER
    PORT_ROUTER += 1
    port = PORT_ROUTER
    iframe = f'<iframe src="http://{GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>'
    process = None

    run_guard = gr.State(False)
    first_load_guard = gr.State(False)

    with gr.Row():
        shutdown_btn = gr.Button(
            "Shutdown Extension", variant="stop", interactive=False
        )
        start_btn = gr.Button("Start Extension", variant="primary", interactive=True)

    def shutdown_extension():
        try:
            nonlocal process, extension_ran
            process.terminate()
            gr.Info("Extension stopped")
            print(f"Terminated {title_name} extension process with PID {process.pid}")
            process = None
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

    @functools.lru_cache(maxsize=1)
    def add_route_once():
        return add_extension_route(package_name, port)

    def run_extension():
        nonlocal extension_ran, process

        yield gr.HTML("Starting extension, please wait..."), gr.skip(), gr.skip()

        print(
            f"\nStarting {title_name} extension in a separate process on port {port}/{package_name}..."
        )
        executable = sys.executable  # "./.venv/Scripts/python.exe"
        executable_venv = f"./.venvs/{package_name}/Scripts/python.exe"
        if os.path.exists(executable_venv):
            executable = executable_venv
        process = subprocess.Popen(
            [executable, "-m", "tts_webui.extensions_loader.proxy_harness"],
            env={
                **os.environ,
                "GRADIO_SERVER_PORT": str(port),
                "GRADIO_ROOT_PATH": f"/{package_name}",
                "TTS_WEBUI_EXTENSION_PACKAGE": package_name,
            },
        )
        # Somehow lift up the errors to gr.Error and yield
        # process.stdout
        extension_ran = True

        if not wait_for_server(port, timeout=120):
            gr.Error(f"Warning: Server on port {port} did not start within timeout")
        add_route_once()
        yield (
            gr.HTML(iframe, padding=False),
            gr.Button(interactive=False),
            gr.Button(interactive=True),
        )
        return

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
