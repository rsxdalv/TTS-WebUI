import os

from tts_webui.gradio_proxy_tree import GradioProxyTree

GRADIO_TREE_PORT = int(os.environ.get("GRADIO_TREE_PORT", 7769))
GRADIO_TREE_HOSTNAME = os.environ.get("GRADIO_TREE_HOSTNAME", "127.0.0.1")
GRADIO_TREE_URL = os.environ.get(
    "GRADIO_TREE_URL", f"http://{GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}"
)
# GRADIO_TREE_URL = os.environ.get("GRADIO_TREE_URL", f"http://{GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}")

gradio_proxy_tree = GradioProxyTree(host="0.0.0.0", port=GRADIO_TREE_PORT)

print(f"Gradio Proxy Tree initialized on {GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}")


def setup_gradio_proxy_tree(gr_options):
    gradio_proxy_tree.add_route("/", gr_options["server_port"])
    gradio_proxy_tree.start_background()


def add_extension_route(package_name, port):
    gradio_proxy_tree.add_route(f"/{package_name}", port)
