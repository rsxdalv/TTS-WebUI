import os
import shutil

import gradio as gr

from tts_webui import core_history_bridge
from tts_webui.core_bridge import bridge


def save_to_favorites(directory: str):
    return bridge.history.copy(directory, "favorites")


def save_to_collection(directory: str, collection: str):
    if core_history_bridge.enabled():
        core_history_bridge.copy(directory, collection)
        return gr.Dropdown(value="Saved")
    shutil.copytree(directory, os.path.join(collection, os.path.basename(directory)))
    return gr.Dropdown(value="Saved")
