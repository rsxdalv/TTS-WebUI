from tts_webui.utils.save_waveform_plot import middleware_save_waveform_plot
from tts_webui.utils.outputs.path import get_relative_output_path_ext


def extension__tts_generation_webui():
    return {
        "package_name": "extension_decorator_save_waveform",
        "name": "Decorator Save Waveform",
        "requirements": "git+https://github.com/rsxdalv/extension_decorator_save_waveform@main",
        "description": "Decorator Save Waveform",
        "extension_type": "decorator",
        "extension_class": "outer",
        "author": "rsxdalv",
        "extension_author": "rsxdalv",
        "license": "MIT",
        "website": "https://github.com/rsxdalv/extension_decorator_save_waveform",
        "extension_website": "https://github.com/rsxdalv/extension_decorator_save_waveform",
        "extension_platform_version": "0.0.1",
    }


def decorator_save_waveform_plot(fn):
    """
    Add waveform_plot to the result_dict.
    """

    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        path = get_relative_output_path_ext(result_dict, ".png")
        print("Saving waveform plot to", path)
        result_dict["waveform_plot"] = middleware_save_waveform_plot(
            result_dict["audio_out"][1], path
        )
        return result_dict

    return wrapper
