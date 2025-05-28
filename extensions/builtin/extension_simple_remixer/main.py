from tts_webui.utils.save_waveform_plot import plot_waveform_as_image
import gradio as gr
import torchaudio
import torch
from gradio_iconbutton import IconButton


def extension__tts_generation_webui():
    simple_remixer_ui()
    return {
        "package_name": "extension_simple_remixer",
        "name": "Simple Remixer",
        "requirements": "git+https://github.com/rsxdalv/extension_simple_remixer@main",
        "description": "Simple remixer allows concatenating multiple audio files and mixing them together.",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_simple_remixer",
        "extension_website": "https://github.com/rsxdalv/extension_simple_remixer",
        "extension_platform_version": "0.0.1",
    }


def simple_remixer_ui():
    input_audio = gr.Audio(label="Input Audio")

    def create_slot(id=0):
        with gr.Group():
            audio = gr.Audio(label=f"Slot {str(id)}")

            with gr.Row():
                clear = IconButton("delete").click(
                    fn=lambda: [gr.Audio(None)],
                    outputs=[audio],
                )
                copy_from_input = IconButton("keyboard_return").click(
                    fn=lambda input_value: [gr.Audio(input_value)],
                    inputs=[input_audio],
                    outputs=[audio],
                )
        return audio

    def slot_stack(i):
        with gr.Column(variant="compact"):
            a = create_slot(i)
            b = create_slot(i)
            c = create_slot(i)
        return a, b, c

    with gr.Row():
        slots = [slot_stack(i) for i in range(3)]
        slots = [x for y in slots for x in y]

    concat = gr.Button("Concatenate")

    output_audio = gr.Audio(label="Output Audio")

    def concat_audio(*slot_audios):
        sample_rate = max(x[0] for x in slot_audios if x is not None)

        resampled_audios = [
            resample_from_to(x[0], sample_rate, x[1]) if x is not None else None
            for x in slot_audios
        ]
        stacked_audios = [
            resampled_audios[i : i + 3] for i in range(0, len(slot_audios), 3)
        ]

        def mix_audio(x):
            non_null_audios = [i for i in x if i is not None]

            if not non_null_audios:
                return None
            max_len = max(i.shape[0] for i in non_null_audios)

            stack = torch.stack(
                [
                    torch.nn.functional.pad(
                        i,
                        (
                            0,
                            max_len - i.shape[0],
                        ),
                    )
                    for i in non_null_audios
                ]
            )
            return torch.sum(
                stack,
                dim=0,
            )

        merged_audios = [mix_audio(x) for x in stacked_audios]
        if non_null_audios := [x for x in merged_audios if x is not None]:
            return gr.Audio((sample_rate, torch.cat(non_null_audios).cpu().numpy()))
        else:
            return gr.Audio(None)

    def resample_from_to(in_sr: int, out_sr: int, in_wav):
        return torchaudio.transforms.Resample(in_sr, out_sr)(
            torch.from_numpy(in_wav).float()
        )

    concat.click(
        fn=concat_audio,
        inputs=slots,  # type: ignore
        outputs=output_audio,
    )

    send_to_input = gr.Button("Send to input")

    send_to_input.click(
        fn=lambda x: gr.Audio(x),
        inputs=output_audio,
        outputs=input_audio,
    )

    return input_audio


def simple_remixer_tab():
    with gr.Tab("Simple Remixer", id="simple_remixer"):
        return simple_remixer_ui()
