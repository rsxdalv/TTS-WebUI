# ruff: noqa: E402
# %%
print("Starting server...\n")

# Apply torch.load monkeypatch early to ensure it's in place before any models are loaded
from tts_webui.utils.torch_load_patch import apply_torch_load_patch

apply_torch_load_patch()
import tts_webui.utils.setup_or_recover as setup_or_recover

setup_or_recover.setup_or_recover()
import tts_webui.utils.dotenv_init as dotenv_init

dotenv_init.init()
import os


def fix_gradio_tabs():
    import gradio as gr
    from gradio_goodtabs import GoodTabs
    from gradio_goodtab import GoodTab as Tab

    gr.Tab = Tab
    gr.Tabs = GoodTabs


fix_gradio_tabs()
from tts_webui.utils.suppress_warnings import suppress_warnings

suppress_warnings()
from tts_webui.config.config import config
from tts_webui.gradio.print_gradio_options import print_gradio_options


gr_options = config["gradio_interface_options"]


def start_gradio_server(config):

    if "--share" in os.sys.argv:
        print("Gradio share mode enabled")
        gr_options["share"] = True

    if "--docker" in os.sys.argv:
        gr_options["server_name"] = "0.0.0.0"
        print("Info: Docker mode: opening gradio server on all interfaces")

    print("Starting Gradio server...")
    if "enable_queue" in gr_options:
        del gr_options["enable_queue"]
    if gr_options["auth"] is not None:
        # split username:password into (username, password)
        gr_options["auth"] = tuple(gr_options["auth"].split(":"))
        print("Gradio server authentication enabled")

    def upgrade_gradio_options(options):
        for key in ["file_directories", "favicon_path", "show_tips"]:
            if key in gr_options:
                del gr_options[key]

    upgrade_gradio_options(gr_options)
    print_gradio_options(gr_options)

    from tts_webui.gradio.main_ui import main_ui

    demo = main_ui(config=config)

    print("\n\n")

    demo.queue().launch(
        **gr_options,
        allowed_paths=["."],
        favicon_path="./react-ui/public/favicon.ico",
    )


def server_hypervisor():
    import subprocess
    import signal
    import sys

    postgres_dir = os.path.join("data", "postgres")

    def stop_postgres(postgres_process):
        try:
            subprocess.check_call(f"pg_ctl stop -D {postgres_dir} -m fast", shell=True)
            print("PostgreSQL stopped gracefully.")
        except Exception as e:
            print(f"Error stopping PostgreSQL: {e}")
            postgres_process.terminate()

    def signal_handler(signal, frame, postgres_process):
        print("Shutting down...")
        stop_postgres(postgres_process)
        sys.exit(0)

    # Check for --no-react flag
    if "--no-react" not in os.sys.argv:
        print("Starting React UI...")
        subprocess.Popen(
            "npm start --prefix react-ui",
            env={
                **os.environ,
                "GRADIO_BACKEND_AUTOMATIC": f"http://127.0.0.1:{gr_options['server_port']}/",
                # "GRADIO_AUTH": gradio_interface_options["auth"].join(":"),
            },
            shell=True,
        )
    else:
        print("Skipping React UI (--no-react flag detected)")

    # Check for --no-database or docker flag
    if "--no-database" in os.sys.argv or "--docker" in os.sys.argv:
        if "--docker" in os.sys.argv:
            print("Info: Docker mode: skipping Postgres")
        else:
            print("Skipping Postgres (--no-database flag detected)")
        return

    print("Starting Postgres...")
    postgres_process = subprocess.Popen(
        f"postgres -D {postgres_dir} -p 7773", shell=True
    )
    try:
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


if __name__ == "__main__":
    server_hypervisor()

    import webbrowser

    if gr_options["inbrowser"] and "--no-react" not in os.sys.argv:
        webbrowser.open("http://localhost:3000")

    start_gradio_server(config=config)
