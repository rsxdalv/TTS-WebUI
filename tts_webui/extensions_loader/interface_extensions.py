import importlib
import importlib.util
from importlib.metadata import version

import gradio as gr

from tts_webui.config.config import config
from tts_webui.core_bridge import bridge
from tts_webui.extensions_loader.extensions_data_loader import (
    filter_extensions_by_type_and_class,
    get_interface_extensions,
)
from tts_webui.extensions_loader.list_tab import extension_list_ui
from tts_webui.extensions_loader.LoadingIndicator import LoadingIndicator
from tts_webui.extensions_loader.setup_proxy_extension import setup_proxy_extension
from tts_webui.utils.generic_error_tab_advanced import generic_error_tab_advanced
from tts_webui.utils.pip_install import (
    pip_install_wrapper,
    pip_uninstall_wrapper,
    venv_setup_wrapper,
)


def check_if_package_installed(package_name, proxy):
    if proxy == "native":
        venv = f".venvs/{package_name}"
        import os

        if not os.path.exists(venv):
            return False
        return True
    else:
        spec = importlib.util.find_spec(package_name)
        return spec is not None


def _get_version(package_name, proxy):
    try:
        if "builtin" in package_name:
            return "Built-in"
        if proxy == "native":
            return ""
            import os
            import subprocess

            venv_python = (
                f".venvs/{package_name}/Scripts/python.exe"
                if os.name == "nt"
                else f".venvs/{package_name}/bin/python"
            )
            result = subprocess.run(
                [
                    venv_python,
                    "-c",
                    f"import importlib.metadata; print(importlib.metadata.version('{package_name}'))",
                ],
                capture_output=True,
                text=True,
            )
            return result.stdout.strip()  # "0.0.3"
        else:
            return f"v{version(package_name)}"
    except Exception as e:
        print(f"Error getting version for {package_name}: {e}")
        return "Unknown"


def _handle_package(package_name, title_name, requirements, proxy=None):
    if package_name in disabled_extensions:
        with LoadingIndicator(title_name, skipped=True):
            with gr.Tab(f"[Disabled] {title_name}"):
                gr.Markdown(f"## {title_name} Extension is disabled")
                enable_btn = gr.Button("Enable")
                enable_btn.click(
                    fn=toggle_extension_state(package_name),
                    outputs=[enable_btn],
                )
        return

    is_installed = check_if_package_installed(package_name, proxy)

    if not is_installed and proxy != "native":
        with gr.Tab(f"[Available] {title_name}"):
            gr.Markdown(f"{title_name} Extension not installed")
            install_btn = gr.Button(
                f"Install {title_name} Extension {'uv venv' if proxy == 'native' else ''}"
            )
            with gr.Accordion("Installation Console", open=True):
                console_text = gr.HTML()
            if proxy == "native":
                install_btn.click(
                    venv_setup_wrapper(requirements, title_name, package_name),
                    outputs=[console_text],
                )
            else:
                install_btn.click(
                    pip_install_wrapper(requirements, title_name),
                    outputs=[console_text],
                )
        return

    with LoadingIndicator(title_name):
        try:
            with gr.Tab(title_name) as tab:
                autostart = package_name in autostart_extensions
                if "builtin" in package_name:
                    pass
                else:
                    install_trigger = _extension_management_ui(
                        package_name,
                        title_name,
                        requirements,
                        version=_get_version(package_name, proxy),
                        show=not is_installed,
                        proxy=proxy,
                        autostart=autostart,
                        is_installed=is_installed,
                    )
                try:
                    if proxy == "native":
                        setup_proxy_extension(
                            package_name,
                            title_name,
                            tab,
                            autostart=autostart,
                            autoload=is_installed,
                            install_trigger=install_trigger,
                            is_installed=is_installed,
                        )
                    else:
                        module = importlib.import_module(f"{package_name}.main")
                        main_tab = getattr(module, "extension__tts_generation_webui")
                        # @gr.render(triggers=[tab.select], trigger_mode="once")
                        # def load_extension():
                        main_tab()
                except Exception as e:
                    generic_error_tab_advanced(
                        e, name=title_name, requirements=requirements
                    )
        except Exception as e:
            generic_error_tab_advanced(e, name=title_name, requirements=requirements)


def enable_extension(package_name):
    def _enable_extension():
        res = bridge.config.extension_enable(package_name)
        print(f"Enabled extension {package_name}")
        gr.Info(res["info"])

    return _enable_extension


def disable_extension(package_name):
    def _disable_extension():
        res = bridge.config.extension_disable(package_name)
        print(f"Disabled extension {package_name}")
        gr.Info(res["info"])

    return _disable_extension


def toggle_extension_state(package_name):
    """Button handler: flips the extension and returns the new button label.

    Go decides from config.json, not from the lists loaded at startup, which
    stay as they were until the restart the toast asks for.
    """

    def _toggle_extension_state():
        res = bridge.config.extension_toggle(package_name)
        gr.Info(res["info"])
        if res["disabled"]:
            print(f"Disabled extension {package_name}")
            return "Enable"
        print(f"Enabled extension {package_name}")
        return "Disable"

    return _toggle_extension_state


def toggle_autostart(package_name):
    def _toggle_autostart(state):
        bridge.config.extension_autostart(package_name, bool(state))
        if state:
            print(f"Added {package_name} to autostart list")
        else:
            print(f"Removed {package_name} from autostart list")

    return _toggle_autostart


def get_latest_version(package_name):
    def _get_latest_version():
        print(f"Getting latest version of {package_name}")
        for line in pip_install_wrapper(
            f"--dry-run --no-deps {package_name}",
            f"{package_name} (dry run, update check)",
        )():
            if "Would install" in line:
                return line.split(" ")[-1]

        return "Already up to date (sometimes incorrect)"

    return _get_latest_version


def _extension_management_ui(
    package_name,
    title_name,
    requirements,
    version,
    show=True,
    proxy=None,
    autostart=False,
    is_installed=True,
):
    with gr.Accordion(f"Manage {title_name} {version} Extension", open=show):
        with gr.Row():
            output = gr.HTML(render=False)
            install_trigger = gr.State(is_installed)
            if proxy == "native":
                venv_install = gr.Button(
                    "Update (uv venv)" if is_installed else "Install (uv venv)",
                    variant="primary",
                )
                venv_install.click(
                    fn=lambda: gr.Button("Updating...", interactive=False),
                    outputs=[venv_install],
                ).then(
                    venv_setup_wrapper(requirements, title_name, package_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Update (uv venv)", interactive=True),
                    outputs=[venv_install],
                ).then(
                    fn=lambda: gr.State(True) if not is_installed else gr.skip(),
                    outputs=[install_trigger],
                )

                start_with_webui_checkbox = gr.Checkbox(
                    value=autostart,
                    label="Start with WebUI",
                )
                start_with_webui_checkbox.change(
                    inputs=[start_with_webui_checkbox],
                    fn=toggle_autostart(package_name),
                )
            else:
                gr.Button("Check for updates").click(
                    get_latest_version(package_name),
                    outputs=[output],
                )

                attempt_update = gr.Button("Update", variant="primary")
                attempt_update.click(
                    fn=lambda: gr.Button("Updating...", interactive=False),
                    outputs=[attempt_update],
                ).then(
                    pip_install_wrapper(requirements, title_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Update", interactive=True),
                    outputs=[attempt_update],
                )

                uninstall_extension_btn = gr.Button("Uninstall", variant="stop")
                uninstall_extension_btn.click(
                    fn=lambda: gr.Button("Uninstalling...", interactive=False),
                    outputs=[uninstall_extension_btn],
                ).then(
                    pip_uninstall_wrapper(package_name, title_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Uninstall", interactive=True),
                    outputs=[uninstall_extension_btn],
                )

            toggle_btn = gr.Button("Disable", variant="secondary")
            toggle_btn.click(
                outputs=[toggle_btn],
                fn=toggle_extension_state(package_name),
            )
            # load_on_open_checkbox = gr.Checkbox(
            #     label="Load on Open", value=True, interactive=False
            # )
            if not proxy == "native":
                check_dependencies_btn = gr.Button(
                    "Check Dependencies", variant="primary"
                )
                check_dependencies_btn.click(
                    fn=lambda: gr.Button("Checking...", interactive=False),
                    outputs=[check_dependencies_btn],
                ).then(
                    pip_install_wrapper(f"--dry-run {requirements}", title_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Check Dependencies", interactive=True),
                    outputs=[check_dependencies_btn],
                )
        with gr.Accordion("Console", open=True):
            output.render()
            return install_trigger


# Get the interface extensions list from the data loader
extension_list_json = get_interface_extensions()
disabled_extensions: list[str] = config.get("extensions", {}).get("disabled", [])
autostart_extensions: list[str] = config.get("extensions", {}).get("autostart", [])
config.setdefault("extensions", {}).setdefault("disabled", disabled_extensions)
config.setdefault("extensions", {}).setdefault("autostart", autostart_extensions)


def handle_extension_class(extension_class, config):
    # Get interface extensions filtered by class from the data loader
    filtered_extensions = filter_extensions_by_type_and_class(
        extension_list_json, "interface", extension_class
    )
    for x in filtered_extensions:
        _handle_package(x["package_name"], x["name"], x["requirements"], x.get("proxy"))


def extension_list_tab():
    extension_list_ui("interface")


if __name__ == "__main__":
    with gr.Blocks() as demo:
        handle_extension_class("audio-music-generation")
        handle_extension_class("audio-conversion")
        handle_extension_class("text-to-speech")

        demo.launch()
