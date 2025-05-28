import gradio as gr
import subprocess
import os


def extension__tts_generation_webui():
    conda_storage_optimizer_ui()
    return {
        "package_name": "extension_conda_storage_optimizer",
        "name": "Conda Storage Optimizer",
        "requirements": "",
        "description": "Conda Storage Optimizer allows cleaning up conda storage to free disk space.",
        "extension_type": "interface",
        "extension_class": "tools",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "",
        "extension_website": "",
        "extension_platform_version": "0.0.1",
    }


def run_conda_command(command):
    """Run a conda command and return the output."""
    try:
        # Check if we're in a conda environment
        if "CONDA_PREFIX" not in os.environ:
            return "Error: No conda environment detected. Please run this tool with conda active."

        # Run the command
        result = subprocess.run(
            command, shell=True, check=True, capture_output=True, text=True
        )
        return result.stdout
    except subprocess.CalledProcessError as e:
        return f"Error: {e.stderr}"
    except Exception as e:
        return f"Error: {str(e)}"


def conda_dry_run():
    return run_conda_command("conda clean --all --dry-run")


def conda_clean_all():
    return run_conda_command("conda clean --all --yes")


def conda_storage_optimizer_ui():
    gr.Markdown("# Conda Storage Optimizer")
    gr.Markdown(
        """This tool helps you clean up conda storage to free disk space.
        
1. First, run a dry run to see what would be cleaned.
2. If you're satisfied with the results, run the actual cleaning.

**Note:** This tool needs to be run with conda active to work properly.
        """
    )

    # Dry run section
    gr.Markdown("## Step 1: Dry Run")
    dry_run_button = gr.Button("Run Dry Run", variant="primary")
    dry_run_output = gr.Textbox(
        label="Dry Run Output",
        placeholder="Click 'Run Dry Run' to see what would be cleaned...",
        lines=10,
    )

    dry_run_button.click(
        fn=conda_dry_run,
        outputs=[dry_run_output],
        api_name="conda_dry_run",
    )

    # Clean section
    gr.Markdown("## Step 2: Clean Storage")
    gr.Markdown(
        """**Warning:** This will permanently delete the files shown in the dry run.
        Make sure you've reviewed the dry run output before proceeding."""
    )
    clean_button = gr.Button("Clean Conda Storage", variant="stop")
    clean_output = gr.Textbox(
        label="Clean Output",
        placeholder="Results will appear here after cleaning...",
        lines=10,
    )

    clean_button.click(
        fn=conda_clean_all,
        outputs=[clean_output],
        api_name="conda_clean_all",
    )


if __name__ == "__main__":
    if "demo" in locals():
        demo.close()

    with gr.Blocks() as demo:
        with gr.Tab("Conda Storage Optimizer"):
            conda_storage_optimizer_ui()

    demo.queue().launch(
        server_port=7771,
    )
