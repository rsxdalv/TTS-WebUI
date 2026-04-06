import os
import shutil
import gradio as gr
from datetime import datetime


VOICES_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "voices")
AUDIO_EXTENSIONS = {".wav", ".mp3", ".flac", ".ogg", ".m4a", ".opus"}


def extension__tts_generation_webui():
    voice_manager_ui()
    return {
        "package_name": "extension_voice_manager",
        "name": "Voice Manager",
        "requirements": "",
        "description": "Upload, browse, and manage voice reference files for TTS models",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "local",
        "extension_author": "local",
        "license": "MIT",
        "website": "",
        "extension_website": "",
        "extension_platform_version": "0.0.1",
    }


def _voices_dir():
    os.makedirs(VOICES_DIR, exist_ok=True)
    return VOICES_DIR


def get_voice_subdirs():
    base = _voices_dir()
    subdirs = ["(root)"]
    for entry in sorted(os.listdir(base)):
        if os.path.isdir(os.path.join(base, entry)):
            subdirs.append(entry)
    return subdirs


def _resolve_subdir(subdir):
    base = _voices_dir()
    if not subdir or subdir == "(root)":
        return base
    target = os.path.join(base, subdir)
    real_target = os.path.realpath(target)
    real_base = os.path.realpath(base)
    if not real_target.startswith(real_base):
        raise ValueError("Invalid subdirectory")
    return target


def get_voice_files(subdir):
    target = _resolve_subdir(subdir)
    if not os.path.isdir(target):
        return []
    rows = []
    for fname in sorted(os.listdir(target)):
        fpath = os.path.join(target, fname)
        if not os.path.isfile(fpath):
            continue
        ext = os.path.splitext(fname)[1].lower()
        if ext not in AUDIO_EXTENSIONS:
            continue
        size = os.path.getsize(fpath)
        mtime = datetime.fromtimestamp(os.path.getmtime(fpath)).strftime("%Y-%m-%d %H:%M")
        if size < 1024:
            size_str = f"{size} B"
        elif size < 1024 * 1024:
            size_str = f"{size / 1024:.1f} KB"
        else:
            size_str = f"{size / (1024 * 1024):.1f} MB"
        rows.append([fname, size_str, mtime])
    return rows


def upload_voices(files, subdir):
    if not files:
        gr.Warning("No files selected for upload.")
        return get_voice_files(subdir)
    target = _resolve_subdir(subdir)
    os.makedirs(target, exist_ok=True)
    count = 0
    for f in files:
        fname = os.path.basename(f)
        ext = os.path.splitext(fname)[1].lower()
        if ext not in AUDIO_EXTENSIONS:
            gr.Warning(f"Skipped {fname}: unsupported format")
            continue
        dest = os.path.join(target, fname)
        shutil.copy2(f, dest)
        count += 1
    gr.Info(f"Uploaded {count} file(s) to voices/{subdir if subdir != '(root)' else ''}")
    return get_voice_files(subdir)


def delete_voice(subdir, selected_file):
    if not selected_file:
        gr.Warning("No file selected.")
        return get_voice_files(subdir), None, ""
    target = _resolve_subdir(subdir)
    fpath = os.path.join(target, selected_file)
    real_fpath = os.path.realpath(fpath)
    real_target = os.path.realpath(target)
    if not real_fpath.startswith(real_target):
        gr.Warning("Invalid file path.")
        return get_voice_files(subdir), None, ""
    if os.path.isfile(fpath):
        os.remove(fpath)
        gr.Info(f"Deleted {selected_file}")
    else:
        gr.Warning(f"File not found: {selected_file}")
    return get_voice_files(subdir), None, ""


def create_subdirectory(name):
    if not name or not name.strip():
        gr.Warning("Please enter a subdirectory name.")
        return get_voice_subdirs(), gr.update()
    name = name.strip()
    if os.sep in name or "/" in name or "\\" in name or ".." in name:
        gr.Warning("Invalid directory name.")
        return get_voice_subdirs(), gr.update()
    target = os.path.join(_voices_dir(), name)
    if os.path.exists(target):
        gr.Warning(f"Directory '{name}' already exists.")
        return get_voice_subdirs(), gr.update()
    os.makedirs(target)
    gr.Info(f"Created subdirectory: {name}")
    return get_voice_subdirs(), gr.update(value=name)


def on_table_select(subdir, evt: gr.SelectData):
    if evt.index[1] != 0:
        return gr.update(), ""
    fname = evt.value
    target = _resolve_subdir(subdir)
    fpath = os.path.join(target, fname)
    if os.path.isfile(fpath):
        return fpath, fname
    return None, ""


def voice_manager_ui():
    gr.Markdown("## Voice Manager")
    gr.Markdown(
        "Upload, browse, and manage voice reference files for TTS models "
        "(Chatterbox, VibeVoice, Index-TTS, etc.)"
    )

    selected_file_state = gr.State("")

    with gr.Row():
        # Left column - browsing and uploading
        with gr.Column(scale=2):
            with gr.Row():
                subdir_dropdown = gr.Dropdown(
                    label="Subdirectory",
                    choices=get_voice_subdirs(),
                    value="(root)",
                    scale=3,
                )
                refresh_btn = gr.Button("Refresh", size="sm", scale=1)

            with gr.Accordion("Create Subdirectory", open=False):
                with gr.Row():
                    new_subdir_input = gr.Textbox(
                        label="New subdirectory name",
                        placeholder="e.g. my-voices",
                        scale=3,
                    )
                    create_subdir_btn = gr.Button("Create", size="sm", scale=1)

            file_upload = gr.File(
                label="Upload voice files",
                file_types=["audio"],
                file_count="multiple",
            )
            upload_btn = gr.Button("Upload to selected subdirectory", variant="primary")

            file_table = gr.Dataframe(
                headers=["Name", "Size", "Date"],
                datatype=["str", "str", "str"],
                value=get_voice_files("(root)"),
                label="Voice Files",
                interactive=False,
                row_count=(1, "dynamic"),
            )

        # Right column - preview and actions
        with gr.Column(scale=1):
            audio_preview = gr.Audio(
                label="Audio Preview",
                type="filepath",
                interactive=False,
            )
            file_info = gr.Textbox(
                label="Selected File",
                interactive=False,
                placeholder="Click a filename in the table to preview",
            )
            delete_btn = gr.Button("Delete Selected File", variant="stop")

    # Event handlers
    def refresh_all(subdir):
        return gr.update(choices=get_voice_subdirs()), get_voice_files(subdir)

    refresh_btn.click(
        fn=refresh_all,
        inputs=[subdir_dropdown],
        outputs=[subdir_dropdown, file_table],
    )

    subdir_dropdown.change(
        fn=get_voice_files,
        inputs=[subdir_dropdown],
        outputs=[file_table],
    ).then(
        fn=lambda: (None, ""),
        outputs=[audio_preview, file_info],
    )

    create_subdir_btn.click(
        fn=create_subdirectory,
        inputs=[new_subdir_input],
        outputs=[subdir_dropdown, subdir_dropdown],
    ).then(
        fn=get_voice_files,
        inputs=[subdir_dropdown],
        outputs=[file_table],
    )

    upload_btn.click(
        fn=upload_voices,
        inputs=[file_upload, subdir_dropdown],
        outputs=[file_table],
    )

    file_table.select(
        fn=on_table_select,
        inputs=[subdir_dropdown],
        outputs=[audio_preview, selected_file_state],
    )

    selected_file_state.change(
        fn=lambda x: x,
        inputs=[selected_file_state],
        outputs=[file_info],
    )

    delete_btn.click(
        fn=delete_voice,
        inputs=[subdir_dropdown, selected_file_state],
        outputs=[file_table, audio_preview, selected_file_state],
    )


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()
    with gr.Blocks() as demo:
        with gr.Tab("Voice Manager"):
            voice_manager_ui()
    demo.launch(server_port=7771)
