import os
import shutil

import gradio as gr

from tts_webui import core_history_bridge


def save_to_favorites(directory: str):
    core_history_bridge.copy(directory, "favorites")
    return gr.Button(value="Saved")


def save_to_collection(directory: str, collection: str):
    if core_history_bridge.enabled():
        core_history_bridge.copy(directory, collection)
        return gr.Dropdown(value="Saved")
    shutil.copytree(directory, os.path.join(collection, os.path.basename(directory)))
    return gr.Dropdown(value="Saved")
