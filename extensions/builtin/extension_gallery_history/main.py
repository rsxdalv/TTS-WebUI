import gradio as gr

from tts_webui.core_bridge import bridge
from tts_webui.history_tab.collections_directories_atom import (
    collections_directories_atom,
    get_collections,
)
from tts_webui.history_tab.delete_generation_cb import delete_generation_cb
from tts_webui.history_tab.save_to_favorites import save_to_favorites
from tts_webui.utils.open_folder import open_collection_folder


def extension__tts_generation_webui():
    history_content(directory="outputs", show_collections=True)
    return {
        "package_name": "extension_gallery_history",
        "name": "Gallery History",
        "requirements": "git+https://github.com/rsxdalv/extension_gallery_history@main",
        "description": "Gallery History allows selecting previously generated audio files by looking at their waveforms",
        "extension_type": "interface",
        "extension_class": "outputs",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_gallery_history",
        "extension_website": "https://github.com/rsxdalv/extension_gallery_history",
        "extension_platform_version": "0.0.1",
    }


audio_list_img = []


def clear_audio():
    return [
        gr.Audio(value=None),
        gr.Image(value=None),
        gr.JSON(value=None),
        gr.Button(visible=False),
    ]


def history_content(directory, show_collections):
    directories = get_collections()
    directory_dropdown = gr.Dropdown(
        value=directory,
        choices=directories,
        label="Select directory of the collection",
        visible=show_collections,
    )
    collections_directories_atom.change(
        fn=lambda x: gr.Dropdown(choices=x),
        inputs=[collections_directories_atom],
        outputs=[directory_dropdown],
    )

    if show_collections:
        create_collection_ui(collections_directories_atom)

    gr.Markdown(
        """
        ### Disclaimer:
        This gallery shows only the files that have an image in the same directory as the audio file.
        The audio generations without an image will not appear at all.
        """
    )

    with gr.Row():
        with gr.Column():
            with gr.Row():
                button_output = gr.Button(value=f"Open collection folder")
                reload_button = gr.Button(value="Refresh", variant="secondary")
            button_output.click(
                lambda x: open_collection_folder(x),
                inputs=[directory_dropdown],
            )

            history_list_as_gallery = gr.Gallery(
                value=[], columns=4, object_fit="contain", height="auto"
            )

        with gr.Column():
            history_bundle_name = gr.Markdown(visible=True)
            folder_root = gr.Textbox(visible=False)
            history_audio = gr.Audio(visible=True, type="filepath", show_label=False)
            history_image = gr.Image(show_label=False)
            history_json = gr.JSON()
            history_npz = gr.Textbox(visible=False)

            with gr.Row():
                delete_from_history = gr.Button(
                    value="Delete", variant="stop", visible=False
                )
                save_to_favorites_history = gr.Button(
                    value="Save to favorites", variant="primary", visible=False
                )
                gr.Markdown("""Use as voice button is now only available in React UI""")
                save_to_voices = gr.Button(
                    value="Save to voices", variant="secondary", visible=False
                )
                open_folder_button = gr.Button(
                    value="Open folder", variant="secondary", visible=False
                )
                open_folder_button.click(open_collection_folder, inputs=folder_root)

                save_to_favorites_history.click(
                    fn=save_to_favorites,
                    inputs=folder_root,
                    outputs=save_to_favorites_history,
                )

                save_to_voices.click(
                    fn=bridge.history.save_to_voices,
                    inputs=history_npz,
                    outputs=save_to_voices,
                )

            save_to_collection_ui(
                directory,
                directories,
                folder_root,
                collections_directories_atom,
            )

    def _select_audio_history(png_path: str):
        # Go decides what each control shows; Python only maps names to components.
        ui = bridge.history.select(png_path)
        return {
            history_bundle_name: ui["bundle_name"],
            folder_root: ui["folder_root"],
            history_audio: ui["audio"],
            history_image: ui["image"],
            history_json: ui["json"],
            history_npz: ui["npz"],
            delete_from_history: ui["delete"],
            save_to_favorites_history: ui["save_to_favorites"],
            save_to_voices: ui["save_to_voices"],
            open_folder_button: ui["open_folder"],
        }

    def select_audio_history2(_list, evt: gr.SelectData):
        return _select_audio_history(audio_list_img[evt.index])  # type: ignore

    outputs = [
        history_bundle_name,
        folder_root,
        history_audio,
        history_image,
        history_json,
        history_npz,
        delete_from_history,
        save_to_favorites_history,
        save_to_voices,
        open_folder_button,
    ]

    history_list_as_gallery.select(
        fn=select_audio_history2,
        inputs=[history_list_as_gallery],
        outputs=outputs,
        preprocess=False,
    )

    def update_history_tab(directory: str):
        global audio_list_img
        audio_list_img = bridge.history.items(directory)
        return gr.Gallery(value=audio_list_img)

    delete_from_history.click(
        fn=clear_audio,
        outputs=[history_audio, history_image, history_json, delete_from_history],
    )
    delete_from_history.click(
        fn=delete_generation_cb(update_history_tab),
        inputs=[folder_root, directory_dropdown],
        outputs=[history_list_as_gallery],
    )

    directory_dropdown.change(
        fn=update_history_tab,
        inputs=[directory_dropdown],
        outputs=[history_list_as_gallery],
    )

    reload_button.click(
        fn=update_history_tab,
        inputs=[directory_dropdown],
        outputs=[history_list_as_gallery],
    )


def save_to_collection_ui(
    directory: str,
    directories: list[str],
    folder_root: gr.Textbox,
    directories_state: gr.JSON,
):
    with gr.Row():
        move_to_collection = gr.Dropdown(
            label="Save to collection",
            choices=directories,
            value=directory,
        )

    move_to_collection.select(
        fn=bridge.history.copy,
        inputs=[folder_root, move_to_collection],
        outputs=[move_to_collection],
    )

    directories_state.change(
        fn=lambda x: gr.Dropdown(choices=x),
        inputs=[directories_state],
        outputs=[move_to_collection],
    )


def create_collection_ui(directories_state):
    new_collection_name = gr.Textbox(label="New collection name", value="")

    def create_collection(new_collection_name):
        res = bridge.history.create_collection(new_collection_name)
        return [res["collections"], res["ui"]]

    create_collection_button = gr.Button(value="Create collection")

    new_collection_name.change(
        fn=lambda: gr.Button(value="Create collection"),
        outputs=[create_collection_button],
    )
    create_collection_button.click(
        fn=create_collection,
        inputs=[new_collection_name],
        outputs=[directories_state, create_collection_button],
    )
