import gradio as gr


def tutorial_tab():
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
    ace_step_demo()
    with gr.Tab("Bark Voice Generation"):
        youtube_video("4LqoEy3BM_0")


def ace_step_demo():
    with gr.Tab("ACE-Step"):
        youtube_video("HFtrCnczZQI")


def youtube_video(video_id, width=560, height=315):
    gr.HTML(
        f"""<iframe width="{width}" height="{height}" src="https://www.youtube.com/embed/{video_id}" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>"""
    )
