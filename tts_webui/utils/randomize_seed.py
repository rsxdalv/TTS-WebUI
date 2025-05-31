import numpy as np
import gradio as gr


def randomize_seed(seed, randomize_seed):
    if randomize_seed:
        return np.random.randint(0, 2**32 - 1, dtype=np.uint32)
    else:
        return int(seed)


def randomize_seed_ui():
    with gr.Row():
        seed_input = gr.Textbox(label="Seed", value="-1")
        randomize_seed_checkbox = gr.Checkbox(label="Randomize seed", value=True)

    return (
        seed_input,
        {
            "fn": randomize_seed,
            "inputs": [seed_input, randomize_seed_checkbox],
            "outputs": [seed_input],
        },
    )
