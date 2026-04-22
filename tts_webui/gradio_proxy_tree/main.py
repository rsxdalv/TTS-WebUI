from tts_webui.gradio_proxy_tree import GradioProxyTree

GRADIO_TREE_PORT = 7769
GRADIO_TREE_HOSTNAME = "127.0.0.1"

gradio_proxy_tree = GradioProxyTree(host="0.0.0.0", port=GRADIO_TREE_PORT)

print(f"Gradio Proxy Tree initialized on {GRADIO_TREE_HOSTNAME}:{GRADIO_TREE_PORT}")

def _setup():
    gradio_proxy_tree.add_route("/main-app", 7770)
    gradio_proxy_tree.start_background()


def add_extension_route(package_name, port):
    gradio_proxy_tree.add_route(f"/{package_name}", port)


_setup()
