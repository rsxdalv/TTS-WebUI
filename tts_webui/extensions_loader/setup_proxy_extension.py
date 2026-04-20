import os
import subprocess

import gradio as gr

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
        time.sleep(0.5)
    return False


from tts_webui.gradio_proxy_tree import GradioProxyTree
GRADIO_TREE_PORT = 7769
GRADIO_TREE_HOSTNAME = "127.0.0.1"
gradio_proxy_tree = GradioProxyTree(host="0.0.0.0", port=GRADIO_TREE_PORT)
gradio_proxy_tree.add_route("/main-app", 7770)

gradio_proxy_tree.start_background()


def setup_proxy_extension(package_name, title_name, tab):
    extension_ran = False
    global PORT_ROUTER
    PORT_ROUTER += 1
    port = PORT_ROUTER
    iframe = f'<iframe src="http://{GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}/{package_name}/" style="width: 100%; height: 100vh;"></iframe>'

    def run_extension():
        nonlocal extension_ran

        yield gr.HTML("Starting extension, please wait...")

        print(
            f"Starting {title_name} extension in a separate process on port {port}/{package_name}..."
        )
        subprocess.Popen(
            "python -m tts_webui.extensions_loader.proxy_harness",
            env={
                **os.environ,
                "GRADIO_SERVER_PORT": str(port),
                "GRADIO_ROOT_PATH": f"/{package_name}",
                "TTS_WEBUI_EXTENSION_PACKAGE": package_name,
            },
            shell=True,
        )
        extension_ran = True

        if not wait_for_server(port):
            gr.Error(f"Warning: Server on port {port} did not start within timeout")
        gradio_proxy_tree.add_route(f"/{package_name}", port)
        yield gr.HTML(iframe, padding=False)
        return

    run_guard = gr.State(False)
    first_load_guard = gr.State(False)

    page = gr.HTML("Loading Extension, please wait...")

    def run_extension_guard(first_load_guard_value):
        if extension_ran:
            if first_load_guard_value:
                return gr.skip(), gr.skip()
            else:
                return gr.skip(), gr.State(True)
        else:
            return gr.State(True), gr.skip()

    first_load_guard.change(fn=lambda: gr.HTML(iframe, padding=False), outputs=page)
    run_guard.change(fn=run_extension, outputs=page)

    tab.select(
        fn=run_extension_guard,
        inputs=[first_load_guard],
        outputs=[run_guard, first_load_guard],
    )
