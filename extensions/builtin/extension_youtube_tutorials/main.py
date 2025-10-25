import gradio as gr


def tutorial_tab():
    # with gr.Tab("TTS WebUI"):
    #     gr.Markdown(
    #         """
    #         # TTS WebUI
    #         ### Coming soon!
    #         """
    #     )
    # with gr.Tab("React UI"):
    #     gr.Markdown(
    #         """
    #         # React UI
    #         ### Coming soon!
    #         """
    #     )
    #
    with gr.Tab("Chat with Chatterbox in SillyTavern!"):
        youtube_video("_0rftbXPJLI")
    # 5:10
    # Now playing
    # Chatterbox TTS by Resemble AI - Local ElevenLabs
    # https://www.youtube.com/watch?v=yJhzwmwFpcs
    with gr.Tab("Chatterbox TTS"):
        youtube_video("yJhzwmwFpcs")
    with gr.Tab("Extensions"):
        youtube_video("https://www.youtube.com/watch?v=nfZEoXOGX5Y")
    # with gr.Tab("TTS WebUI scroll through preview 2025 (silent)"):
    #     youtube_video("https://www.youtube.com/watch?v=SSmxO-ccCXE")
    with gr.Tab("ACE-Step Demo"):
        youtube_video("HFtrCnczZQI")
    with gr.Tab("Bark Voice Generation"):
        youtube_video("4LqoEy3BM_0")


def youtube_video(video_id, width="100%", height="500px"):
    if video_id.startswith("https://"):
        video_id = video_id.split("v=")[1]
    gr.HTML(
        f"""<iframe width="{width}" height="{height}" src="https://www.youtube.com/embed/{video_id}" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>"""
    )


def extension__tts_generation_webui():
    tutorial_tab()
    return {
        "package_name": "extensions.builtin.extension_youtube_tutorials",
        "name": "YouTube Tutorials",
        "requirements": "git+https://github.com/rsxdalv/tts_webui_extension.youtube_tutorials@main",
        "description": "YouTube Tutorials shows a list of YouTube tutorials in the web UI",
        "extension_type": "interface",
        "extension_class": "tutorials",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/tts_webui_extension.youtube_tutorials",
        "extension_website": "https://github.com/rsxdalv/tts_webui_extension.youtube_tutorials",
        "extension_platform_version": "0.0.1",
    }


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()  # type: ignore
    with gr.Blocks() as demo:
        tutorial_tab()
    demo.launch()
