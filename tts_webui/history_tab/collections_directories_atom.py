import gradio as gr

from tts_webui.core_bridge import bridge


def get_collections():
    return bridge.history.collections()


collections_directories_atom = gr.JSON(
    visible=False, value=get_collections(), render=False
)
