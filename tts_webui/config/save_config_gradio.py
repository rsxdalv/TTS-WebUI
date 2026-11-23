from typing import Any, List

from tts_webui.config.config import config
from tts_webui.core_bridge import bridge


def _convert_empty_strings_to_none(dictionary: dict):
    for key, value in dictionary.items():
        if value == "":
            dictionary[key] = None


def _recreate_ui_dict(keys: List[str], inputs: List[Any]):
    return {keys[i]: value for i, value in enumerate(inputs)}


def _generate_gradio_settings(keys: List[str], inputs: List[Any]):
    gradio_interface_options = _recreate_ui_dict(keys, inputs)
    _convert_empty_strings_to_none(gradio_interface_options)
    gradio_interface_options["server_port"] = int(
        gradio_interface_options["server_port"]
    )

    # Ensure auth is always saved as a string, not as a tuple/list
    if "auth" in gradio_interface_options:
        auth_value = gradio_interface_options["auth"]
        if isinstance(auth_value, (tuple, list)):
            gradio_interface_options["auth"] = ":".join(auth_value)

    return gradio_interface_options


def save_config_gradio(keys: List[str], inputs: List[Any]):
    options = _generate_gradio_settings(keys, inputs)
    bridge.config.replace_section("gradio_interface_options", options)
    config["gradio_interface_options"] = options

    return f"Saved {config['gradio_interface_options']}"
