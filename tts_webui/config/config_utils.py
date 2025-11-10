import json
import os
from typing import Any, Dict, Optional, Union

from tts_webui.config._save_config import _save_config
from tts_webui.config.config import config


def get_config_value(namespace: str, key: str, default: Any = None) -> Any:
    """
    Safely get a value from config with namespace support.

    Args:
        namespace: The namespace (section) in config
        key: The key to retrieve
        default: Default value if key doesn't exist

    Returns:
        The value or default if not found
    """
    try:
        if namespace not in config:
            return default
        if key not in config[namespace]:
            return default
        return config[namespace][key]
    except (KeyError, TypeError):
        return default


def set_config_value(namespace: str, key: str, value: Any) -> None:
    """
    Safely set a value in config with namespace support and save to disk.

    Args:
        namespace: The namespace (section) in config
        key: The key to set
        value: The value to store

    Returns:
        None
    """
    if namespace not in config:
        config[namespace] = {}

    config[namespace][key] = value

    _save_config(config)
