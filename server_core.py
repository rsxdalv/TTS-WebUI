"""Entry point the Go launcher runs for the Gradio child.

Installs the config bridge (a no-op unless TTS_CORE_URL is set, which only the
launcher sets) and then runs the thin Gradio entry (tts_webui.core_server).
Routing, extension processes, .env and the output folders are the launcher's.
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    if HERE not in sys.path:
        sys.path.insert(0, HERE)

    # First: config.py loads config.json at import.
    from tts_webui.core_config_bridge import install as install_config

    install_config()

    from tts_webui.core_server import main as run_server

    run_server()


if __name__ == "__main__":
    main()
