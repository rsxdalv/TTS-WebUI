from tts_webui.core_bridge import bridge


def save_to_favorites(directory: str):
    return bridge.history.copy(directory, "favorites")


def save_to_collection(directory: str, collection: str):
    return bridge.history.copy(directory, collection)
