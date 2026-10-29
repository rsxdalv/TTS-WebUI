import functools
import os
import subprocess
import sys
import urllib.error
import urllib.request

import gradio as gr

from tts_webui.config.config_utils import get_config_value
from tts_webui.gradio_proxy_tree.main import (
    GRADIO_TREE_URL,
    add_extension_route,
)

PORT_ROUTER = 27770

# When this env var is set the Go core is running; delegate engine start/stop to it.
_CORE_URL = os.environ.get("TTS_WEBUI_CORE_URL", "").rstrip("/")
_CORE_TOKEN = os.environ.get("TTS_WEBUI_TOKEN", "")


def _core_request(method: str, path: str) -> dict:
    """Send a JSON request to the Go core API. Returns the parsed response dict."""
    url = f"{_CORE_URL}{path}"
    req = urllib.request.Request(
        url,
        method=method,
        headers={
            "X-TTS-Token": _CORE_TOKEN,
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            import json

            return json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f"core API {method} {url} → {exc.code}: {exc.reason}") from exc


def get_openai_api_host():
    port = get_config_value("extension_openai_tts_api", "port", 7778)
    return f"http://localhost:{port}"


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
    if _CORE_URL:
        return _setup_proxy_extension_via_core(
            package_name,
            title_name,
            tab,
            autostart=autostart,
            autoload=autoload,
            install_trigger=install_trigger,
            is_installed=is_installed,
        )
    return _setup_proxy_extension_local(
        package_name,
        title_name,
        tab,
        autostart=autostart,
        autoload=autoload,
        install_trigger=install_trigger,
        is_installed=is_installed,
    )


def _setup_proxy_extension_via_core(
    package_name,
    title_name,
    tab,
    autostart=False,
    autoload=True,
    install_trigger=None,
    is_installed=False,
):
    """Extension lifecycle managed by the Go core via its engine API."""
    engine_started = False
    proxy_url = None

    with gr.Row():
        shutdown_btn = gr.Button(
            "Shutdown Extension", variant="stop", interactive=False
        )
        start_btn = gr.Button("Start Extension", variant="primary", interactive=True)

    page = gr.HTML(
        "Please install extension"
        if not is_installed
        else "Loading Extension, please wait..."
        if autoload
        else "Press 'Start Extension'"
    )

    run_guard = gr.State(False)
    first_load_guard = gr.State(False)

    def shutdown_extension():
        nonlocal engine_started
        try:
            _core_request("POST", f"/api/engines/{package_name}/stop")
            engine_started = False
            gr.Info("Extension stopped")
            return (
                gr.HTML("Extension stopped."),
                gr.Button(interactive=True),
                gr.Button(interactive=False),
            )
        except Exception as exc:
            gr.Error(f"Error stopping extension: {exc}")
            return gr.skip(), gr.skip(), gr.skip()

    def run_extension():
        nonlocal engine_started, proxy_url

        yield gr.HTML("Starting extension via core, please wait…"), gr.skip(), gr.skip()

        try:
            result = _core_request("POST", f"/api/engines/{package_name}/start")
        except Exception as exc:
            yield gr.HTML(f"Failed to start engine: {exc}"), gr.skip(), gr.skip()
            return

        # The core returns {"running": true, "port": N}
        port = result.get("port", 0)
        if port:
            proxy_url = f"http://127.0.0.1:{port}"
        else:
            proxy_url = result.get("url", "")

        engine_started = True

        # Register the route with the Python proxy tree so the iframe works.
        if proxy_url:
            add_extension_route(package_name, port)

        iframe = f'<iframe src="{GRADIO_TREE_URL}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>'
        yield (
            gr.HTML(iframe, padding=False),
            gr.Button(interactive=False),
            gr.Button(interactive=True),
        )

    def run_extension_guard(first_load_guard_value):
        if engine_started:
            if first_load_guard_value:
                return gr.skip(), gr.skip()
            else:
                return gr.skip(), gr.State(True)
        else:
            return gr.State(True), gr.skip()

    first_load_guard.change(
        fn=lambda: gr.HTML(
            f'<iframe src="{GRADIO_TREE_URL}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>',
            padding=False,
        ),
        outputs=page,
    )
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

    if install_trigger is not None:
        gr.on(triggers=[install_trigger.change], fn=lambda: None).then(
            run_extension_guard,
            inputs=[first_load_guard],
            outputs=[run_guard, first_load_guard],
        )


def _setup_proxy_extension_local(
    package_name,
    title_name,
    tab,
    autostart=False,
    autoload=True,
    install_trigger=None,
    is_installed=False,
):
    """Original local-subprocess extension lifecycle (no TTS_WEBUI_CORE_URL)."""
    extension_ran = False
    global PORT_ROUTER
    PORT_ROUTER += 1
    port = PORT_ROUTER
    iframe = f'<iframe src="{GRADIO_TREE_URL}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>'
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
        executable = sys.executable
        venv_python = (
            f".venvs/{package_name}/Scripts/python.exe"
            if os.name == "nt"
            else f".venvs/{package_name}/bin/python"
        )
        if os.path.exists(venv_python):
            executable = venv_python
        process = subprocess.Popen(
            [executable, "-m", "tts_webui.extensions_loader.proxy_harness"],
            env={
                **os.environ,
                "GRADIO_SERVER_PORT": str(port),
                "GRADIO_ROOT_PATH": f"/{package_name}",
                "TTS_WEBUI_EXTENSION_PACKAGE": package_name,
                "OPENAI_PROXY_HOST": get_openai_api_host(),
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
