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
import gradio as gr

import gradio as gr
from gradio_goodtabs import GoodTabs
from gradio_goodtab import GoodTab as Tab
gr.Tab = Tab
gr.Tabs = GoodTabs

from tts_webui.utils.suppress_warnings import suppress_warnings

suppress_warnings()
from tts_webui.config.load_config import default_config
from tts_webui.config.config import config
from tts_webui.css.css import full_css
from tts_webui.history_tab.collections_directories_atom import (
    collections_directories_atom,
)
from tts_webui.utils.generic_error_tab_advanced import generic_error_tab_advanced
from tts_webui.extensions_loader.interface_extensions import (
    extension_list_tab,
    handle_extension_class,
)
from tts_webui.extensions_loader.decorator_extensions import (
    extension_decorator_list_tab,
)
from tts_webui.utils.print_gradio_options import print_gradio_options


def reload_config_and_restart_ui():
    os._exit(0)
    # print("Reloading config and restarting UI...")
    # config = load_config()
    # gradio_interface_options = config["gradio_interface_options"] if "gradio_interface_options" in config else {}
    # demo.close()
    # time.sleep(1)
    # demo.launch(**gradio_interface_options)


gradio_interface_options = (
    config["gradio_interface_options"]
    if "gradio_interface_options" in config
    else default_config
)


def run_tab(module_name, function_name, name, requirements=None):
    import time
    import importlib

    print(f"Loading {name} tab...", end="")
    start_time = time.time()
    try:
        module = importlib.import_module(module_name)
        func = getattr(module, function_name)
        func()
    except Exception as e:
        generic_error_tab_advanced(e, name=name, requirements=requirements)
    finally:
        elapsed_time = time.time() - start_time
        print(f" done in {elapsed_time:.2f} seconds.")


def load_tabs(list_of_tabs):
    for tab in list_of_tabs:
        module_name, function_name, name = tab[:3]
        requirements = tab[3] if len(tab) > 3 else None
        run_tab(module_name, function_name, name, requirements)


def main_ui(theme_choice="Base"):
    themes = {
        "Base": gr.themes.Base,
        "Default": gr.themes.Default,
        "Monochrome": gr.themes.Monochrome,
    }
    theme: gr.themes.Base = themes[theme_choice](
        # primary_hue="blue",
        primary_hue="sky",
        secondary_hue="sky",
        neutral_hue="neutral",
        font=[
            gr.themes.GoogleFont("Inter"),
            "ui-sans-serif",
            "system-ui",
            "sans-serif",
        ],
    )
    theme.set(
        embed_radius="*radius_sm",
        block_label_radius="*radius_sm",
        block_label_right_radius="*radius_sm",
        block_radius="*radius_sm",
        block_title_radius="*radius_sm",
        container_radius="*radius_sm",
        checkbox_border_radius="*radius_sm",
        input_radius="*radius_sm",
        table_radius="*radius_sm",
        button_large_radius="*radius_sm",
        button_small_radius="*radius_sm",
        button_primary_background_fill_hover="*primary_300",
        button_primary_background_fill_hover_dark="*primary_600",
        button_secondary_background_fill_hover="*secondary_200",
        button_secondary_background_fill_hover_dark="*secondary_600",
    )

    with gr.Blocks(
        css=full_css,
        title="TTS WebUI",
        analytics_enabled=False,  # it broke too many times
        theme=theme,
    ) as blocks:
        gr.Markdown(
            """
            # TTS WebUI
            ### (This is the Gradio UI with more tools but more basic UI) | [React UI](http://localhost:3000) | [Feedback / Bug reports](https://forms.gle/2L62owhBsGFzdFBC8) | [Discord Server](https://discord.gg/V8BKTVRtJ9)
            """
        )
        with gr.Tabs():
            all_tabs()

    return blocks


def all_tabs():
    with gr.Tab("💬 Text-to-Speech"), gr.Tabs():
        handle_extension_class("text-to-speech", config)
    with gr.Tab("🎼 Audio/Music Generation"), gr.Tabs():
        handle_extension_class("audio-music-generation", config)
    with gr.Tab("🎙️ Audio Conversion"), gr.Tabs():
        handle_extension_class("audio-conversion", config)
    with gr.Tab("📁 Outputs"), gr.Tabs():
        from tts_webui.history_tab.main import history_tab

        collections_directories_atom.render()
        try:
            history_tab()
            history_tab(directory="favorites")
            history_tab(
                directory="outputs",
                show_collections=True,
            )
        except Exception as e:
            generic_error_tab_advanced(e, name="History", requirements=None)

        outputs_tabs = []
        load_tabs(outputs_tabs)

        handle_extension_class("outputs", config)

    with gr.Tab("🔧 Tools"), gr.Tabs():
        handle_extension_class("tools", config)
    with gr.Tab("⚙️ Settings"), gr.Tabs():
        from tts_webui.settings_tab_gradio import settings_tab_gradio

        settings_tab_gradio(reload_config_and_restart_ui, gradio_interface_options)

        settings_tabs = [
            (
                "tts_webui.utils.model_location_settings_tab",
                "model_location_settings_tab",
                "Model Location Settings",
            ),
            ("tts_webui.utils.gpu_info_tab", "gpu_info_tab", "GPU Info"),
            ("tts_webui.utils.pip_list_tab", "pip_list_tab", "Installed Packages"),
        ]
        load_tabs(settings_tabs)

        extension_list_tab()
        extension_decorator_list_tab()

        handle_extension_class("settings", config)
    with gr.Tab("📚 Tutorials"), gr.Tabs():
        # from tts_webui.utils.tutorial_tab import tutorial_tab

        # tutorial_tab()
        with gr.Tab("TTS WebUI"):
            gr.Markdown(
                """
                # TTS WebUI
                ### Coming soon!
                """
            )
        with gr.Tab("React UI"):
            gr.Markdown(
                """
                # React UI
                ### Coming soon!
                """
            )


def start_gradio_server():

    # detect if --share is passed
    if "--share" in os.sys.argv:
        print("Gradio share mode enabled")
        gradio_interface_options["share"] = True

    if "--docker" in os.sys.argv:
        gradio_interface_options["server_name"] = "0.0.0.0"
        print("Info: Docker mode: opening gradio server on all interfaces")

    print("Starting Gradio server...")
    if "enable_queue" in gradio_interface_options:
        del gradio_interface_options["enable_queue"]
    if gradio_interface_options["auth"] is not None:
        # split username:password into (username, password)
        gradio_interface_options["auth"] = tuple(
            gradio_interface_options["auth"].split(":")
        )
        print("Gradio server authentication enabled")
    # delete show_tips option
    if "show_tips" in gradio_interface_options:
        del gradio_interface_options["show_tips"]
    # TypeError: Blocks.launch() got an unexpected keyword argument 'file_directories'
    if "file_directories" in gradio_interface_options:
        del gradio_interface_options["file_directories"]
    print_gradio_options(gradio_interface_options)

    demo = main_ui()

    print("\n\n")

    if gradio_interface_options["server_name"] == "0.0.0.0":
        print("Notice: Server is open to the internet")
        print(
            f"Gradio server will be available on http://localhost:{gradio_interface_options['server_port']}"
        )

    # concurrency_count=gradio_interface_options.get("concurrency_count", 5),
    demo.queue().launch(**gradio_interface_options, allowed_paths=["."])


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
                "GRADIO_BACKEND_AUTOMATIC": f"http://127.0.0.1:{gradio_interface_options['server_port']}/",
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

    if gradio_interface_options["inbrowser"] and "--no-react" not in os.sys.argv:
        webbrowser.open("http://localhost:3000")

    start_gradio_server()
