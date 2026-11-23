"""Gradio-only server entry for the Go core (replaces server.py's start()).

The Go core has already created outputs/ and favorites/, written .env when it
was missing, loaded .env/.env.user into this process's environment, started the
router, React and the extension processes, and decided how Gradio is launched
(core/internal/serverpy/launchopts). This module only does the work that must
happen inside the interpreter: patches, config, the Gradio app and its launch.

It does not set up a proxy tree, start React or a database, or open a browser.
"""


def launch_kwargs(options):
    """Go's launch options as ``launch()`` takes them.

    JSON has no tuples: auth arrives as a list of [user, password] pairs, which
    Gradio wants as tuples.
    """
    options = dict(options)
    if options.get("auth"):
        options["auth"] = [tuple(pair) for pair in options["auth"]]
    return options


def main():
    print("Starting TTS WebUI... ", end="")

    from tts_webui.utils.suppress_warnings import suppress_warnings
    from tts_webui.utils.torch_load_patch import apply_torch_load_patch

    apply_torch_load_patch()
    suppress_warnings()

    from tts_webui.config.config import config
    from tts_webui.core_bridge import bridge

    launch = bridge.launch.options()
    for line in launch["lines"]:
        print(line)
    kwargs = launch_kwargs(launch["options"])

    from tts_webui.gradio.blocks import main_block

    demo = main_block(config=config)
    try:
        demo.queue().launch(**kwargs)
        demo.block_thread()
    except Exception as e:
        print(f"Failed to launch Gradio server: {e}")


if __name__ == "__main__":
    main()
