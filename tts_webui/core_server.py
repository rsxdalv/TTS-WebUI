"""Gradio-only server entry for the Go core (replaces server.py's start()).

The Go core has already created outputs/ and favorites/, written .env when it
was missing, loaded .env/.env.user, started the router, React and the
extension processes. This module only does the work that must happen inside
the interpreter: patches, config, the Gradio app and its launch on
TTS_GRADIO_HOST:TTS_GRADIO_PORT (applied to the config by core_router_bridge).

It does not set up a proxy tree, start React or a database, or open a browser.
"""

import os

FAVICON_PATH = "./react-ui/public/favicon.ico"
_DROPPED_OPTIONS = (
    "file_directories",
    "favicon_path",
    "show_tips",
    "enable_queue",
    "prevent_thread_lock",
)


def upgrade_gradio_options(options):
    """server.py's upgrade_gradio_options: auth to a tuple, drop old keys.

    Mutates and returns ``options``.
    """
    auth = options.get("auth")
    if auth is not None:
        if isinstance(auth, str):
            options["auth"] = tuple(auth.split(":"))
        elif isinstance(auth, (list, tuple)):
            options["auth"] = tuple(auth)
        print("Gradio server authentication enabled")
    for key in _DROPPED_OPTIONS:
        options.pop(key, None)
    return options


def launch_options(gr_options):
    """The keyword arguments for ``demo.queue().launch``."""
    options = upgrade_gradio_options(gr_options)
    options["inbrowser"] = False  # the Go launcher opens the browser
    return {
        **options,
        "allowed_paths": ["."],
        "favicon_path": FAVICON_PATH,
        "prevent_thread_lock": True,
    }


def main():
    print("Starting TTS WebUI... ", end="")

    import tts_webui.dotenv_manager.init as dotenv_init
    from tts_webui.utils.suppress_warnings import suppress_warnings
    from tts_webui.utils.torch_load_patch import apply_torch_load_patch

    dotenv_init.init()
    apply_torch_load_patch()
    suppress_warnings()

    from tts_webui.config.config import config
    from tts_webui.gradio.print_gradio_options import print_gradio_options

    kwargs = launch_options(config["gradio_interface_options"])
    print_gradio_options(kwargs)

    from tts_webui.gradio.blocks import main_block

    demo = main_block(config=config)
    try:
        demo.queue().launch(**kwargs)
        demo.block_thread()
    except Exception as e:
        print(f"Failed to launch Gradio server: {e}")


if __name__ == "__main__":
    main()
