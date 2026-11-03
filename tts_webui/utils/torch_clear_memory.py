import gc


def torch_clear_memory():
    try:
        import torch  # lazy — torch is optional (P5)
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
    except Exception:
        pass
    gc.collect()


# Re-exported from tts-webui-sdk (P2)
try:
    from tts_webui_sdk.torch_load_patch import torch_clear_memory as _sdk_torch_clear_memory  # noqa: F401
except ImportError:
    pass
