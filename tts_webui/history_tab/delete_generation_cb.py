import shutil

from tts_webui import core_history_bridge


def delete_generation_cb(refresh):
    def delete_generation(directory: str, *args):
        if core_history_bridge.enabled():
            core_history_bridge.delete(directory)
        else:
            shutil.rmtree(directory)
        return refresh(*args)

    return delete_generation
