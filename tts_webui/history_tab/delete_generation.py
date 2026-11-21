from tts_webui.core_bridge import bridge


def delete_generation(directory: str):
    """Delete a generation folder (the hidden "Delete (API ONLY)" button).

    Go does the work: the path comes from the UI, so it goes through the same
    checks as every other path (it must be a generation folder inside
    outputs/, favorites/ or collections/<name>).
    """
    return bridge.history.delete(directory=directory)
