import importlib

import gradio as gr

from tts_webui.core_bridge import bridge
from tts_webui.extensions_loader.list_tab import extension_list_ui
from tts_webui.extensions_loader.LoadingIndicator import LoadingIndicator
from tts_webui.extensions_loader.setup_proxy_extension import setup_proxy_extension
from tts_webui.utils.generic_error_tab_advanced import generic_error_tab_advanced


def _is_builtin_tab(ext):
    """A built-in tab is part of the host. A built-in entry that is "native"
    (extension_model_downloader) has its code in the checkout but runs in its
    own venv, which is installed and updated like any other native entry."""
    return "builtin" in ext["package_name"] and ext["proxy"] != "native"


def _version_label(ext):
    """What the Manage accordion shows: Go reads the version from the main env."""
    if ext["proxy"] == "native" or _is_builtin_tab(ext):
        return ""
    return f"v{ext['version']}" if ext.get("version") else "Unknown"


def _stream(operation, package_name):
    """A Gradio handler that streams one Go install operation for this extension.

    Go installs, updates and checks by package name: it reads the requirements
    and the proxy kind from the extension lists, and runs pip or uv itself.
    """

    def run():
        yield from operation(package_name)

    return run


def _handle_package(ext):
    """Builds the tab of one extension from Go's entry (see bridge.extcatalog.resolve)."""
    package_name, title_name = ext["package_name"], ext["name"]
    requirements, proxy = ext["requirements"], ext["proxy"]
    if not ext["enabled"]:
        with LoadingIndicator(title_name, skipped=True):
            with gr.Tab(f"[Disabled] {title_name}"):
                gr.Markdown(f"## {title_name} Extension is disabled")
                enable_btn = gr.Button("Enable")
                enable_btn.click(
                    fn=toggle_extension_state(package_name),
                    outputs=[enable_btn],
                )
        return

    is_installed = ext["installed"]

    if not is_installed and proxy != "native":
        with gr.Tab(f"[Available] {title_name}"):
            gr.Markdown(f"{title_name} Extension not installed")
            install_btn = gr.Button(f"Install {title_name} Extension ")
            with gr.Accordion("Installation Console", open=True):
                console_text = gr.HTML()
            install_btn.click(
                _stream(bridge.extcatalog.install, package_name),
                outputs=[console_text],
            )
        return

    with LoadingIndicator(title_name):
        try:
            with gr.Tab(title_name) as tab:
                autostart = ext["autostart"]
                if _is_builtin_tab(ext):
                    pass
                else:
                    install_trigger = _extension_management_ui(
                        package_name,
                        title_name,
                        version=_version_label(ext),
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
                        e,
                        name=title_name,
                        requirements=requirements,
                        package_name=package_name,
                    )
        except Exception as e:
            generic_error_tab_advanced(
                e,
                name=title_name,
                requirements=requirements,
                package_name=package_name,
            )


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
        return bridge.extcatalog.latest_version(package_name)

    return _get_latest_version


def _extension_management_ui(
    package_name,
    title_name,
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
                    _stream(bridge.extcatalog.install, package_name),
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
                    _stream(bridge.extcatalog.install, package_name),
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
                    _stream(bridge.extcatalog.uninstall, package_name),
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
                    _stream(bridge.extcatalog.check_dependencies, package_name),
                    outputs=[output],
                ).then(
                    fn=lambda: gr.Button("Check Dependencies", interactive=True),
                    outputs=[check_dependencies_btn],
                )
        with gr.Accordion("Console", open=True):
            output.render()
            return install_trigger


def handle_extension_class(extension_class):
    """Go lists the extensions of the class in tab order, with their state."""
    for ext in bridge.extcatalog.resolve("interface", extension_class)["extensions"]:
        _handle_package(ext)


def extension_list_tab():
    extension_list_ui("interface")
