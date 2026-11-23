"""Entry point used by the Go core instead of server.py.

Installs the Go bridges (each is a no-op unless TTS_CORE_URL is set) and then
runs the thin Gradio entry (tts_webui.core_server); server.py stays as the
legacy path. Extension processes are started through bridge.extensions.
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    if HERE not in sys.path:
        sys.path.insert(0, HERE)

    # First: config.py loads config.json at import, and the router bridge imports it.
    from tts_webui.core_config_bridge import install as install_config

    install_config()

    from tts_webui.core_router_bridge import install as install_router

    install_router()

    from tts_webui.core_server import main as run_server

    run_server()


if __name__ == "__main__":
    main()
