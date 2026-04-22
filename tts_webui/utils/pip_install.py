import os
import re
import subprocess

from tts_webui.utils.get_torch_command import get_torch_command


def write_log(output, name, type):
    script_dir = os.path.dirname((__file__))
    logs_dir = os.path.join(script_dir, "..", "..", "installer_scripts", "logs")
    os.makedirs(logs_dir, exist_ok=True)

    # Sanitize the name to be safe for filenames
    safe_name = re.sub(r'[<>:"/\\|?*]', "_", name)

    with open(os.path.join(logs_dir, f"{type}-{safe_name}.log"), "w") as outfile:
        outfile.write("\n".join(output))


def pip_install_wrapper(requirements, name, include_gradio=True):
    def fn():
        output = []
        command = f"{requirements} gradio==5.49.1" if include_gradio else requirements
        for line in _pip_install(command, name):
            output.append(str(line))
            yield "<br />".join(output)

        write_log(output, name, type="pip-install")
        return line
        # verify installation by importing the package or drying run install

    return fn


def venv_setup_wrapper(requirements, name, package_name):
    def fn():
        output = []
        venv = f".venvs/{package_name}"
        torch = get_torch_command()
        torchcodec = (
            "torchcodec --index-url=https://download.pytorch.org/whl/cpu"  # safe option
        )
        xformers = "xformers==0.0.35"  # no index-url needed
        compatibility = '"gradio<=5.49.1" "gradio-goodtabs>=0.0.5" "gradio-goodtab>=0.0.5" "gradio-iconbutton>=0.0.1" "ffmpeg-python==0.2.0" "matplotlib"'
        uv_install_cmd = f"uv pip install --python {venv}/Scripts/python.exe"
        commands = [
            f"uv venv {venv} --allow-existing",
            f"{uv_install_cmd} {torch}",
            f"{uv_install_cmd} {torchcodec}",
            f"{uv_install_cmd} {xformers}",
            f"{uv_install_cmd} {requirements} {compatibility}",
        ]
        for cmd in commands:
            for line in _stream_shell_command(cmd):
                output.append(str(line))
                yield "<br />".join(output)

        message = (
            f"\nSuccessfully set up virtual environment for {name} with dependencies\n"
        )
        print(message)
        yield message
        output.append(message)

        write_log(output, name, type="uv-venv-install")
        return line

    return fn


def pip_uninstall_wrapper(package_name, name):
    def fn():
        output = []
        for line in _pip_uninstall(package_name, name):
            output.append(line)
            yield "<br />".join(output)

        write_log(output, name, type="pip-uninstall")

    return fn


def _pip_install(requirements, name):
    # process = subprocess.Popen(
    #     f"uv pip install {requirements}",
    #     shell=True,
    #     stdout=subprocess.PIPE,
    #     stderr=subprocess.STDOUT,
    #     universal_newlines=True,
    # )

    # # Stream the output to the console
    # for line in process.stdout:  # type: ignore
    #     print(line, end="")
    #     yield line

    # # Wait for the process to finish
    # process.wait()

    # # Check if the process was successful
    # if process.returncode == 0:
    #     print(f"Successfully installed {name}")
    #     yield f"Successfully installed {name}, please restart the webui"
    # else:
    #     print(f"Failed to install {name}")
    #     yield f"Failed to install {name}"
    try:
        print(f"Installing {name} dependencies...")
        yield from _stream_shell_command(f"pip install {requirements}")
        print(f"Successfully installed {name} dependencies")
        yield f"Successfully installed {name} dependencies"
        yield "Please restart the webui to see the changes"
        yield True
    except Exception:
        print(f"Failed to install {name} dependencies")
        yield f"Failed to install {name} dependencies"
        yield False


def _pip_uninstall(package_name, name):
    try:
        print(f"Uninstalling {name} ({package_name})...")
        yield from _stream_shell_command(f"pip uninstall -y {package_name}")
        # yield from _stream_shell_command(f"uv pip uninstall {package_name}")
        print(f"Successfully uninstalled {name} ({package_name})")
        yield f"Successfully uninstalled {name} ({package_name})"
    except Exception:
        print(f"Failed to uninstall {name} ({package_name})")
        yield f"Failed to uninstall {name} ({package_name})"


def _stream_shell_command(command):
    process = subprocess.Popen(
        command,
        shell=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        universal_newlines=True,
    )

    # Stream the output to the console
    for line in process.stdout:  # type: ignore
        print(line, end="")
        yield line

    # Wait for the process to finish
    process.wait()

    if process.returncode != 0:
        raise Exception(f"Failed to run {command}")
