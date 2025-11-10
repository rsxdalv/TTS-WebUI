import os
import tts_webui.dotenv_manager.init as dotenv_init

from tts_webui.config.config import config
from tts_webui.gradio.print_gradio_options import print_gradio_options
from tts_webui.utils.suppress_warnings import suppress_warnings
from tts_webui.utils.torch_load_patch import apply_torch_load_patch

def create_output_folders():
    if not os.path.exists("outputs"):
        os.makedirs("outputs")
    if not os.path.exists("favorites"):
        os.makedirs("favorites")

print("Starting TTS WebUI... ", end="")
def tts_webui_init_environment():
    create_output_folders()
    dotenv_init.init()
    apply_torch_load_patch()
    suppress_warnings()

tts_webui_init_environment()

argv = os.sys.argv
gr_options = config["gradio_interface_options"]
REACT_UI_PORT = os.environ.get("REACT_UI_PORT", 3000)


def start_gradio_server(gr_options, config):

    if "--share" in argv:
        print("Gradio share mode enabled")
        gr_options["share"] = True

    if "--docker" in argv:
        gr_options["server_name"] = "0.0.0.0"
        print("Info: Docker mode: opening gradio server on all interfaces")

    def upgrade_gradio_options(options):
        if gr_options["auth"] is not None:
            # split username:password into (username, password)
            gr_options["auth"] = tuple(gr_options["auth"].split(":"))
            print("Gradio server authentication enabled")
        for key in ["file_directories", "favicon_path", "show_tips", "enable_queue"]:
            if key in options:
                del options[key]
        return options

    parsed_options = upgrade_gradio_options(gr_options)
    print_gradio_options(parsed_options)

    from tts_webui.gradio.blocks import main_block

    demo = main_block(config=config)

    try:
        demo.queue().launch(
            **parsed_options,
            allowed_paths=["."],
            favicon_path="./react-ui/public/favicon.ico",
        )
    except Exception as e:
        print(f"Failed to launch Gradio server: {e}")


def server_hypervisor():
    import subprocess
    import signal
    import sys

    if "--no-react" not in argv:
        print("\nStarting React UI...")
        subprocess.Popen(
            f"npm start --prefix react-ui -- -p {REACT_UI_PORT}",
            env={
                **os.environ,
                "GRADIO_BACKEND_AUTOMATIC": f"http://127.0.0.1:{gr_options['server_port']}/",
                # "GRADIO_AUTH": gradio_interface_options["auth"].join(":"),
            },
            shell=True,
        )
    else:
        print("skipping React UI (--no-react flag detected) ", end="")

    if "--no-database" in argv or "--docker" in argv:
        if "--no-database" in argv:
            print("skipping Postgres (--no-database flag detected) \n", end="")
        return

    print("\nStarting Postgres...")
    postgres_dir = os.path.join("data", "postgres")
    postgres_process = subprocess.Popen(
        f"postgres -D {postgres_dir} -p 7773", shell=True
    )
    try:

        def stop_postgres(postgres_process):
            try:
                subprocess.check_call(
                    f"pg_ctl stop -D {postgres_dir} -m fast", shell=True
                )
                print("PostgreSQL stopped gracefully.")
            except Exception as e:
                print(f"Error stopping PostgreSQL: {e}")
                postgres_process.terminate()

        def signal_handler(signal, frame, postgres_process):
            print("Shutting down...")
            stop_postgres(postgres_process)
            sys.exit(0)

        signal.signal(
            signal.SIGINT,
            lambda sig, frame: signal_handler(sig, frame, postgres_process),
        )  # Ctrl+C
        signal.signal(
            signal.SIGTERM,
            lambda sig, frame: signal_handler(sig, frame, postgres_process),
        )  # Termination signals
        if os.name != "nt":
            signal.signal(
                signal.SIGHUP,
                lambda sig, frame: signal_handler(sig, frame, postgres_process),
            )  # Terminal closure
            signal.signal(
                signal.SIGQUIT,
                lambda sig, frame: signal_handler(sig, frame, postgres_process),
            )  # Quit
    except (ValueError, OSError) as e:
        print(f"Failed to set signal handlers: {e}")


def start():
    server_hypervisor()

    import webbrowser

    if gr_options["inbrowser"] and "--no-react" not in os.sys.argv:
        webbrowser.open(f"http://localhost:{REACT_UI_PORT}")
    start_gradio_server(gr_options=gr_options, config=config)


if __name__ == "__main__":
    start()
