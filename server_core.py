"""Entry point used by the Go core instead of server.py.

Installs the Go bridges (each is a no-op unless TTS_CORE_URL is set) and then
runs server.py unchanged.
"""

import os
import runpy
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    if HERE not in sys.path:
        sys.path.insert(0, HERE)

    from tts_webui.core_router_bridge import install as install_router

    install_router()

    try:
        from tts_webui.core_ext_bridge import install as install_ext
    except ModuleNotFoundError as e:
        # Only "the bridge does not exist"; a broken bridge must still fail loudly.
        if e.name not in ("tts_webui.core_ext_bridge", "tts_webui"):
            raise
    else:
        install_ext()

    runpy.run_path(os.path.join(HERE, "server.py"), run_name="__main__")


if __name__ == "__main__":
    main()
