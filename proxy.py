"""
Test harness for GradioProxyTree.
Maps /mms -> localhost:20001
"""

import logging
from tts_webui.gradio_proxy_tree import GradioProxyTree

logging.basicConfig(level=logging.INFO)

tree = GradioProxyTree(host="0.0.0.0", port=8079)
tree.add_route("/mms", 20001)

if __name__ == "__main__":
    tree.start_background()

    # wait 30s
    import time
    time.sleep(3)

    tree.add_route("/mms2", 20002)

    time.sleep(9999)