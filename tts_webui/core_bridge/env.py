from .base import POST, SubBridge


class EnvBridge(SubBridge):
    """The .env file: ``/api/env/*`` in core/internal/serverpy/appenv.

    - save_model_locations(huggingface_hub_cache, hf_home, torch_home,
      xdg_cache_home): Go rewrites .env with the default text and these four
      values (an empty one is written commented out) and answers "saved". Go is
      the only writer of .env; it refuses values that are not one line.
    """

    name = "env"
    ops = {
        "save_model_locations": (
            POST,
            ("huggingface_hub_cache", "hf_home", "torch_home", "xdg_cache_home"),
        ),
    }
