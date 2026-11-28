import functools
import importlib
import json
import os
import time
from typing import Literal

import gradio as gr

from tts_webui.core_bridge import bridge

# Set by the Go core for extension processes, which have no control API (and no
# token): the resolved decorators of both classes, {"outer": ..., "inner": ...}.
# Same name as extcatalog.DecoratorsEnv.
DECORATORS_ENV = "TTS_WEBUI_DECORATORS"


def _get_pretty_time(elapsed):
    seconds = f"{elapsed:.2f}"
    seconds = seconds.replace("0.", " .")
    return f"{seconds.rjust(6, ' ')} seconds."


def _resolve(class_name: str):
    """Go's answer for the decorators of one class (see bridge.extcatalog.resolve)."""
    handed_over = os.environ.get(DECORATORS_ENV)
    if handed_over:
        return json.loads(handed_over)[class_name]
    return bridge.extcatalog.resolve("decorator", class_name)


def _load_decorators(class_name: Literal["outer", "inner"]):
    """
    Loads all decorators from extensions.

    The decorators are loaded from the "main" module of the extension.
    Decorators must be functions prefixed with "decorator_".
    Generators are detected by the suffix "_generator".

    For example:
    def decorator_save_ogg(fn):
        def wrapper(*args, **kwargs):
            return fn(*args, **kwargs)
        return wrapper

    def decorator_save_ogg_generator(fn):
        def wrapper(*args, **kwargs):
            yield from fn(*args, **kwargs)
        return wrapper

    Go decides which extensions are listed, in which order and which are
    disabled; the decorator functions a module defines are only known here, so
    the disabled function names (from the same list) are applied here.

    Args:
        class_name (str): "outer" or "inner"

    Returns:
        wrappers (list): List of decorators.
        gen_wrappers (list): List of decorators for generators.
    """
    wrappers = []
    gen_wrappers = []
    resolved = _resolve(class_name)
    disabled_names = set(resolved.get("disabled_names", []))

    def _load(x: dict):
        module = importlib.import_module(f"{x['package_name']}.main")
        names = [n for n in dir(module) if n.startswith("decorator_")]
        loaded = [n for n in names if n not in disabled_names]
        skipped = [n for n in names if n in disabled_names]
        for name in loaded:
            wrapper = getattr(module, name)
            if name.endswith("_generator"):
                gen_wrappers.append(wrapper)
            else:
                wrappers.append(wrapper)
        return loaded, skipped

    for x in resolved["extensions"]:
        if not x["enabled"]:
            print(f"Skipping disabled decorator extension {x['name']}")
            continue
        print(f"Loading decorator {x['name'].ljust(30, '.')}...", end="")
        start_time = time.time()
        try:
            loaded, skipped = _load(x)
            print(_get_pretty_time(time.time() - start_time))

            if loaded:
                print(f"  Loaded: {', '.join(loaded)}")
            if skipped:
                print(f"  Skipped: {', '.join(skipped)}")
        except Exception as e:
            print(f"Failed to load: {e}")

    wrappers.reverse()
    gen_wrappers.reverse()
    return wrappers, gen_wrappers


OUTER_WRAPPERS, OUTER_WRAPPERS_GEN = _load_decorators("outer")
INNER_WRAPPERS, INNER_WRAPPERS_GEN = _load_decorators("inner")


def _create_decorator(wrappers_list):
    def decorator(fn0):
        for wrapper in wrappers_list:
            fn0 = wrapper(fn0)

        @functools.wraps(fn0)
        def wrapped(*args, **kwargs):
            return fn0(*args, **kwargs)

        return wrapped

    return decorator


def _create_decorator_generator(wrappers_list):
    def decorator(fn0):
        for wrapper in wrappers_list:
            fn0 = wrapper(fn0)

        @functools.wraps(fn0)
        def wrapped(*args, **kwargs):
            yield from fn0(*args, **kwargs)

        return wrapped

    return decorator


# Define the four decorators using the helper function
decorator_extension_outer = _create_decorator(OUTER_WRAPPERS)
decorator_extension_inner = _create_decorator(INNER_WRAPPERS)
decorator_extension_outer_generator = _create_decorator_generator(OUTER_WRAPPERS_GEN)
decorator_extension_inner_generator = _create_decorator_generator(INNER_WRAPPERS_GEN)
