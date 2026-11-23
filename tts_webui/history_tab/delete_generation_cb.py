from tts_webui.core_bridge import bridge


def delete_generation_cb(refresh):
    def delete_generation(directory: str, *args):
        bridge.history.delete(directory=directory)
        return refresh(*args)

    return delete_generation
