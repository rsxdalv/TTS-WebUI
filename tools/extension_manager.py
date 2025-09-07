#!/usr/bin/env python
print("Starting Extension Management UI...\n")
# Import necessary modules
import os
import sys
import gradio as gr
import importlib

# Add the project root directory to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from tts_webui.gradio.css import full_css

from tts_webui.extensions_loader.interface_extensions import (
    extension_list_tab,
    get_latest_version,
)
from tts_webui.extensions_loader.extensions_data_loader import get_interface_extensions
from tts_webui.extensions_loader.decorator_extensions import (
    extension_decorator_list_tab,
)
from tts_webui.utils.pip_install import pip_install_wrapper, pip_uninstall_wrapper
from tts_webui.config.config import config
from tts_webui.config._save_config import _save_config


# Helper functions for extension management
def get_installed_extensions(extension_list_json):
    """Get a list of installed extensions"""
    installed_packages = []
    for x in extension_list_json:
        package_name = x["package_name"]
        try:
            if importlib.util.find_spec(package_name) is not None:
                installed_packages.append((package_name, x["name"], x))
        except:
            pass
    return installed_packages


def get_disabled_extensions():
    """Get the list of disabled extensions from config"""
    try:
        return config["extensions"]["disabled"]
    except KeyError:
        # Initialize if not present
        if "extensions" not in config:
            config["extensions"] = {}
        if "disabled" not in config["extensions"]:
            config["extensions"]["disabled"] = []
        return config["extensions"]["disabled"]


def save_disabled_extensions(disabled_list):
    """Save the disabled extensions list to config"""
    if "extensions" not in config:
        config["extensions"] = {}
    config["extensions"]["disabled"] = disabled_list
    _save_config(config)
    return f"Saved disabled extensions: {disabled_list}"


def toggle_extension_state(package_name, disabled_list):
    """Toggle the enabled/disabled state of an extension"""
    if package_name in disabled_list:
        disabled_list.remove(package_name)
        return disabled_list, f"Enabled {package_name}"
    else:
        disabled_list.append(package_name)
        return disabled_list, f"Disabled {package_name}"


def main():
    with gr.Blocks(
        css=full_css,
        title="TTS WebUI - Extension Manager",
        analytics_enabled=False,
        theme=gr.themes.Base(
            primary_hue="sky",
            secondary_hue="sky",
            neutral_hue="neutral",
            font=[
                gr.themes.GoogleFont("Inter"),
                "ui-sans-serif",
                "system-ui",
                "sans-serif",
            ],
        ),
    ) as blocks:
        gr.Markdown(
            """
            # TTS WebUI - Extension Manager
            ### Manage your extensions without starting the full WebUI
            """
        )

        # Hidden check updates functionality (not shown in UI but accessible via code)
        updates_output = gr.HTML(visible=False)

        def check_all_updates():
            extension_list_json = get_interface_extensions()
            installed_packages = []
            for x in extension_list_json:
                package_name = x["package_name"]
                try:
                    if importlib.util.find_spec(package_name) is not None:
                        installed_packages.append((package_name, x["name"]))
                except:
                    pass

            results = ["<h3>Update Check Results:</h3>", "<ul>"]
            for package_name, display_name in installed_packages:
                try:
                    latest_version = ""
                    for line in get_latest_version(package_name)():
                        latest_version = line
                    results.append(
                        f"<li><b>{display_name}</b> ({package_name}): {latest_version}</li>"
                    )
                except Exception as e:
                    results.append(
                        f"<li><b>{display_name}</b> ({package_name}): Error checking version - {str(e)}</li>"
                    )

            results.append("</ul>")
            return "".join(results)

        # Extension management UI
        with gr.Tabs():
            extensions_by_category_tab()
            quick_actions_tab()
            extension_list_tab()
            extension_decorator_list_tab()

    share = "--share" in sys.argv
    if share:
        print("Launching with public link via Gradio Share...")
    else:
        print("Launching on local host only...")

    gradio_interface_options = {
        "server_name": "127.0.0.1",
        "server_port": 7771,  # User preferred port
        "inbrowser": True,
        "share": share,
    }
    blocks.queue().launch(**gradio_interface_options, allowed_paths=["."])


def quick_actions_tab():
    """Tab for quick installation and uninstallation of extensions"""
    extension_list_json = get_interface_extensions()

    # Get list of installed extensions (excluding built-in)
    installed_packages = []
    for x in extension_list_json:
        package_name = x["package_name"]
        try:
            if importlib.util.find_spec(
                package_name
            ) is not None and not package_name.startswith("extensions.builtin"):
                installed_packages.append((x["name"], package_name))
        except:
            pass

    # Create a list of external extensions (non-builtin)
    external_extension_list = [
        x for x in extension_list_json if "builtin" not in x["package_name"]
    ]

    # Format extension names for dropdowns
    install_choices = [(x["name"], x["package_name"]) for x in external_extension_list]

    with gr.Tab("Quick Actions"):
        gr.Markdown("## Quick Extension Management")
        gr.Markdown(
            "Install or uninstall extensions quickly without browsing through categories."
        )

        with gr.Row():
            with gr.Column():
                gr.Markdown("### Quick Install")
                install_dropdown = gr.Dropdown(
                    label="Select Extension to Install",
                    choices=install_choices,
                    type="value",
                )
                install_button = gr.Button("Install Extension", variant="primary")
                install_output = gr.HTML()

                def install_extension(package_name):
                    if not package_name:
                        return "Please select an extension to install"
                    requirements = [
                        x["requirements"]
                        for x in external_extension_list
                        if x["package_name"] == package_name
                    ][0]
                    yield from pip_install_wrapper(requirements, package_name)()

                install_button.click(
                    fn=install_extension,
                    inputs=[install_dropdown],
                    outputs=[install_output],
                )

            with gr.Column():
                gr.Markdown("### Quick Uninstall")
                uninstall_dropdown = gr.Dropdown(
                    label="Select Extension to Uninstall",
                    choices=installed_packages,
                    type="value",
                )
                uninstall_button = gr.Button("Uninstall Extension", variant="stop")
                uninstall_output = gr.HTML()

                def uninstall_extension(package_name):
                    if not package_name:
                        return "Please select an extension to uninstall"
                    yield from pip_uninstall_wrapper(package_name, package_name)()

                uninstall_button.click(
                    fn=uninstall_extension,
                    inputs=[uninstall_dropdown],
                    outputs=[uninstall_output],
                )


def extensions_by_category_tab():
    """Tab showing extensions organized by category"""
    extension_list_json = get_interface_extensions()
    disabled_extensions = get_disabled_extensions()

    # Get list of installed extensions
    installed_packages = []
    for x in extension_list_json:
        package_name = x["package_name"]
        try:
            if importlib.util.find_spec(package_name) is not None:
                installed_packages.append(package_name)
        except:
            pass

    with gr.Tab("Extensions"):
        gr.Markdown("## Extension Categories")

        # Display extensions by category in tabs
        with gr.Tabs():
            for ext_class in [
                "text-to-speech",
                "audio-music-generation",
                "audio-conversion",
                "tools",
                "outputs",
            ]:
                display_category_tab(
                    ext_class,
                    extension_list_json,
                    installed_packages,
                    disabled_extensions,
                )


def get_category_extensions(
    extension_list_json, ext_class, installed_packages, disabled_extensions
):
    """Get extensions for a specific category"""
    category_extensions = []

    # Collect all extensions for this category
    for x in extension_list_json:
        if x["extension_type"] == "interface" and x["extension_class"] == ext_class:
            package_name = x["package_name"]
            is_installed = package_name in installed_packages
            is_disabled = package_name in disabled_extensions
            # Add to category list with installation and disabled status
            category_extensions.append((x, is_installed, is_disabled))

    # Sort extensions - installed first, then alphabetically by name
    category_extensions.sort(key=lambda item: (not item[1], item[0]["name"].lower()))

    return category_extensions


def display_category_tab(
    ext_class, extension_list_json, installed_packages, disabled_extensions
):
    """Display a tab for a specific extension category"""
    with gr.Tab(f"{ext_class.replace('-', ' ').title()}"):
        category_extensions = get_category_extensions(
            extension_list_json, ext_class, installed_packages, disabled_extensions
        )

        # Display message if no extensions in this category
        if not category_extensions:
            gr.Markdown(
                f"No {ext_class.replace('-', ' ').title()} extensions available"
            )
            return

        # Display all extensions for this category
        for x, is_installed, is_disabled in category_extensions:
            display_extension_ui(x, is_installed, is_disabled)


def display_extension_ui(x, is_installed, is_disabled):
    """Display UI for a single extension"""
    package_name = x["package_name"]
    with gr.Group():
        with gr.Row():
            with gr.Column(scale=4):
                gr.Markdown(f"### {x['name']}")
                gr.Markdown(f"**Description:** {x['description']}")
                gr.Markdown(f"**Package:** {package_name}")
                gr.Markdown(f"**Website:** [{x['website']}]({x['website']})")

                # Show status
                if is_installed:
                    if is_disabled:
                        gr.Markdown("**Status:** 🚫 disabled")
                    else:
                        gr.Markdown("**Status:** ✅ installed")
                else:
                    gr.Markdown("**Status:** ❌ not installed")

            with gr.Column(scale=1):
                # Check if this is a built-in extension
                is_builtin = package_name.startswith("extensions.builtin")

                def install_extension():
                    yield from pip_install_wrapper(x["requirements"], package_name)()

                if is_installed:
                    if not is_builtin:
                        # Update button (only for non-builtin extensions)
                        update_btn = gr.Button("Update", variant="primary")
                        update_btn.click(
                            fn=install_extension,
                            outputs=[gr.HTML()],
                        )

                        def uninstall_extension():
                            yield from pip_uninstall_wrapper(
                                package_name, package_name
                            )()

                        # Uninstall button (only for non-builtin extensions)
                        uninstall_btn = gr.Button("Uninstall", variant="stop")
                        uninstall_btn.click(
                            fn=uninstall_extension,
                            outputs=[gr.HTML()],
                        )

                    # Toggle enable/disable button (for all extensions)
                    toggle_btn = gr.Button(
                        "Enable" if is_disabled else "Disable",
                        variant="secondary",
                    )

                    def toggle_extension(package_name=package_name):
                        disabled_list = get_disabled_extensions()
                        new_list, message = toggle_extension_state(
                            package_name, disabled_list
                        )
                        save_disabled_extensions(new_list)
                        return (
                            message
                            + "<br>Please restart the application for changes to take effect."
                        )

                    toggle_btn.click(
                        fn=toggle_extension,
                        outputs=[gr.HTML()],
                    ).then(
                        fn=lambda button_text: gr.Button(
                            "Enable" if button_text == "Disable" else "Disable"
                        ),
                        inputs=[toggle_btn],
                        outputs=[toggle_btn],
                    )

                    if is_builtin:
                        gr.Markdown(
                            "*Built-in extension: cannot be updated or uninstalled*"
                        )
                else:
                    # Install button
                    install_btn = gr.Button("Install", variant="primary")
                    install_btn.click(
                        fn=install_extension,
                        outputs=[gr.HTML()],
                    )

        gr.Markdown("---")


if __name__ == "__main__":
    main()
