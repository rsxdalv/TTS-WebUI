import gradio as gr
import importlib.metadata


def pip_list_tab():
    pip_list = gr.Markdown("Press Refresh to load the list")

    gr.Button("Refresh").click(fn=refresh_pip_list, outputs=pip_list)

    gr.Button("API_GET_PIP_LIST", visible=False).click(
        fn=get_pip_list,
        outputs=[gr.JSON(None, visible=False)],
        api_name="get_pip_list",
    )


def get_pip_list():
    packages = [
        {
            "name": x.name,
            "version": x.version,
        }
        for x in importlib.metadata.distributions()
    ]
    return packages


def render_pip_list(pip_list):
    if isinstance(pip_list, list):
        return "\n\n".join([f"{x['name']}=={x['version']}" for x in pip_list])
    else:
        return pip_list


def refresh_pip_list():
    return render_pip_list(get_pip_list())


def extension__tts_generation_webui():
    pip_list_tab()
    return {
        "package_name": "extension_pip_list",
        "name": "Installed Packages",
        "requirements": "git+https://github.com/rsxdalv/tts_webui_extension.pip_list@main",
        "description": "Pip List shows the list of installed packages in the web UI",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts_webui_extension.pip_list",
        "extension_website": "https://github.com/rsxdalv/tts_webui_extension.pip_list",
        "extension_platform_version": "0.0.1",
    }


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()  # type: ignore
    with gr.Blocks() as demo:
        pip_list_tab()

    demo.launch()
