from .base import GET, POST, SubBridge


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history.

    What Go answers, per operation:
    - collections: ["outputs", "favorites", "collections/<name>", ...]
    - items(collection): the .png paths that have a .wav next to them
    - rows(directory): the rows of the History table, one list per generation
      folder: [date, name, when, path]
    - select(path): {bundle_name, folder_root (plain string), audio, image, json,
      npz, delete, save_to_favorites, save_to_voices, open_folder}, controls as components
    - create_collection(name): {"collections": [...], "ui": the "Created" Button}
    - delete(directory): {"ok": true}
    - copy(directory, to): the "Saved" Button (favorites) or Dropdown (others)
    - save_to_voices(npz): the "Saved" Button
    """

    name = "history"
    ops = {
        "collections": (GET, ()),
        "items": (GET, ("collection",)),
        "rows": (GET, ("directory",)),
        "select": (GET, ("path",)),
        "create_collection": (POST, ("name",)),
        "delete": (POST, ("directory",)),
        "copy": (POST, ("directory", "to")),
        "save_to_voices": (POST, ("npz",)),
    }
