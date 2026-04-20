"""
Gradio Proxy Tree - Dynamic reverse proxy for Gradio apps.

Provides a single FastAPI server that can proxy multiple Gradio instances,
each mounted at a different path prefix pointing to a different local port.

Usage:
    from tts_webui.gradio_proxy_tree import GradioProxyTree

    tree = GradioProxyTree(host="0.0.0.0", port=8079)
    tree.add_route("/mms", 20001)
    tree.add_route("/bark", 20002)
    tree.start()  # blocks, or use tree.start_background()
"""

from .proxy_tree import GradioProxyTree

__all__ = ["GradioProxyTree"]
