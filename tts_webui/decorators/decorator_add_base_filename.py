import os
from datetime import datetime
from tts_webui.utils.prompt_to_title import prompt_to_title

output_path = "outputs"


def format_filename(title, model, date):
    return f"{date}__{model}__{title}"


def format_date_for_file(date: datetime):
    return date.strftime("%Y-%m-%d_%H-%M-%S")


def _create_base_filename(kwargs, result_dict):
    prompt = kwargs.get("text", "")
    is_long = result_dict.get("long", False)
    base_filename = format_filename(
        title=prompt_to_title(prompt) + ("_long" if is_long else ""),
        model=prompt_to_title(kwargs["_type"]),
        date=format_date_for_file(result_dict["date"]),
    )
    result_dict["filename"] = base_filename
    result_dict["folder_root"] = os.path.join(output_path, base_filename)
    os.makedirs(result_dict["folder_root"], exist_ok=True)
    return result_dict


def decorator_add_base_filename(fn):
    """
    Add filename and folder_root to the result_dict, and create the folder_root directory.
    """

    def wrapper(*args, **kwargs):
        result_dict = fn(*args, **kwargs)
        return _create_base_filename(kwargs, result_dict)

    return wrapper


def decorator_add_base_filename_generator(fn):
    """
    Add filename and folder_root to the result_dict, and create the folder_root directory.
    """

    def wrapper(*args, **kwargs):
        for result_dict in fn(*args, **kwargs):
            if result_dict is None:
                continue
            yield _create_base_filename(kwargs, result_dict)

    return wrapper
