import importlib
import importlib.util
from importlib.metadata import version

import gradio as gr

from tts_webui.config._save_config import _save_config
from tts_webui.config.config import config
from tts_webui.extensions_loader.extensions_data_loader import (
    filter_extensions_by_type_and_class,
    get_interface_extensions,
)
from tts_webui.extensions_loader.LoadingIndicator import LoadingIndicator
from tts_webui.extensions_loader.setup_proxy_extension import setup_proxy_extension
from tts_webui.utils.generic_error_tab_advanced import generic_error_tab_advanced
from tts_webui.utils.pip_install import (
    pip_install_wrapper,
    pip_uninstall_wrapper,
    venv_setup_wrapper,
)


def uninstall_extension(package_name):
    yield from pip_uninstall_wrapper(package_name, package_name)()


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


def _handle_package(package_name, title_name, requirements, proxy=None):
    if package_name in disabled_extensions:
        with LoadingIndicator(title_name, skipped=True):
            with gr.Tab(f"[Disabled] {title_name}"):
                gr.Markdown(f"## {title_name} Extension is disabled")
                enable_btn = gr.Button("Enable")
                enable_btn.click(
                    fn=toggle_extension_state(package_name, disabled_extensions),
                    outputs=[enable_btn],
                )
        return

    if not check_if_package_installed(package_name, proxy):
        with gr.Tab(f"[Available] {title_name}"):
            gr.Markdown(f"{title_name} Extension not installed")
            install_btn = gr.Button(f"Install {title_name} Extension {'uv venv' if proxy == 'native' else ''}")
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
                    _extension_management_ui(
                        package_name,
                        title_name,
                        requirements,
                        version=(
                            "0.0.1"
                            if "builtin" in package_name or proxy == "native"
                            else version(package_name)
                        ),
                        show=False,
                        proxy=proxy,
                        autostart=autostart,
                    )
                try:
                    if proxy == "native":
                        setup_proxy_extension(
                            package_name,
                            title_name,
                            tab,
                            autostart=autostart,
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
        disabled_extensions.remove(package_name)
        print(f"Enabled extension {package_name}")
        gr.Info(
            "Enabled extension. Please restart the application for changes to take effect."
        )
        _save_config(config)

    return _enable_extension


def disable_extension(package_name):
    def _disable_extension():
        disabled_extensions.append(package_name)
        print(f"Disabled extension {package_name}")
        gr.Info(
            "Disabled extension. Please restart the application for changes to take effect."
        )
        _save_config(config)

    return _disable_extension


def toggle_extension_state(package_name, disabled_list):
    def _toggle_extension_state():
        if package_name in disabled_list:
            enable_extension(package_name)()
            return "Disable"
        else:
            disable_extension(package_name)()
            return "Enable"

    return _toggle_extension_state


def toggle_autostart(package_name, autostart_list):
    def _toggle_autostart(state):
        if state:
            autostart_list.append(package_name)
            print(f"Added {package_name} to autostart list")
        else:
            autostart_list.remove(package_name)
            print(f"Removed {package_name} from autostart list")
        _save_config(config)

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
    package_name, title_name, requirements, version, show=True, proxy=None, autostart=False
):
    with gr.Accordion(f"Manage {title_name} v{version} Extension", open=show):
        output = gr.HTML(render=False)
        with gr.Row():
            gr.Button("Check for updates").click(
                get_latest_version(package_name),
                outputs=[output],
            )
            if proxy == "native":
                venv_install = gr.Button("Attempt Update (uv venv)", variant="primary")
                venv_install.click(
                    fn=lambda: gr.Button("Updating...", interactive=False),
                    outputs=[venv_install],
                ).then(
                    venv_setup_wrapper(requirements, title_name, package_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Attempt Update (uv venv)", interactive=True),
                    outputs=[venv_install],
                )
            else:
                attempt_update = gr.Button("Attempt Update", variant="primary")
                attempt_update.click(
                    fn=lambda: gr.Button("Updating...", interactive=False),
                    outputs=[attempt_update],
                ).then(
                    pip_install_wrapper(requirements, title_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Attempt Update", interactive=True),
                    outputs=[attempt_update],
                )
            uninstall_extension_btn = gr.Button("Uninstall Extension", variant="stop")
            uninstall_extension_btn.click(
                fn=lambda: gr.Button("Uninstalling...", interactive=False),
                outputs=[uninstall_extension_btn],
            ).then(
                pip_uninstall_wrapper(package_name, title_name),
                outputs=[output],
            ).then(
                fn=lambda: gr.Button("Uninstall Extension", interactive=True),
                outputs=[uninstall_extension_btn],
            )
            toggle_btn = gr.Button("Disable", variant="secondary")
            toggle_btn.click(
                outputs=[toggle_btn],
                fn=toggle_extension_state(package_name, disabled_extensions),
            )
            start_with_webui_checkbox = gr.Checkbox(
                value=autostart,
                label="Start with WebUI",
            )
            start_with_webui_checkbox.change(
                inputs=[start_with_webui_checkbox],
                fn=toggle_autostart(package_name, autostart_extensions),
            )
            # load_on_open_checkbox = gr.Checkbox(
            #     label="Load on Open", value=True, interactive=False
            # )
            check_dependencies_btn = gr.Button("Check Dependencies", variant="primary")
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
    gr.Markdown("List of all extensions")
    table_string = """| Title | Description |\n| --- | --- |\n"""
    for x in extension_list_json:
        table_string += (
            # f"| {x['name']} (v{x['version']}) "
            f"| {x['name']} "
            + f"| {x['description']} (website: {x['website']}) (extension_website: {x['extension_website']}) |\n"
        )
    gr.Markdown(table_string)

    external_extension_list = [
        x for x in extension_list_json if "builtin" not in x["package_name"]
    ]
    # sort
    external_extension_list = sorted(
        external_extension_list, key=lambda x: x["name"].lower()
    )

    with gr.Row():
        with gr.Column():
            gr.Markdown("Install/Uninstall Extensions")

            install_dropdown = gr.Dropdown(
                label="Select Extension to Install",
                choices=[x["package_name"] for x in external_extension_list],
            )

            install_button = gr.Button("Install extension")

            def install_extension(package_name):
                requirements = [
                    x["requirements"]
                    for x in external_extension_list
                    if x["package_name"] == package_name
                ][0]
                yield from pip_install_wrapper(requirements, package_name)()

            install_button.click(
                fn=install_extension,
                inputs=[install_dropdown],
                outputs=[gr.HTML()],
                api_name="install_extension",
            )

        with gr.Column():
            gr.Markdown("Uninstall Extensions")
            uninstall_dropdown = gr.Dropdown(
                label="Select Extension to Uninstall",
                choices=[x["package_name"] for x in external_extension_list],
            )
            uninstall_button = gr.Button("Uninstall extension")

            uninstall_button.click(
                fn=uninstall_extension,
                inputs=[uninstall_dropdown],
                outputs=[gr.HTML()],
                api_name="uninstall_extension",
            )


if __name__ == "__main__":
    with gr.Blocks() as demo:
        handle_extension_class("audio-music-generation")
        handle_extension_class("audio-conversion")
        handle_extension_class("text-to-speech")

        demo.launch()
