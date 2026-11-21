from .base import SubBridge


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history.

    Each method forwards its arguments by name to the Go endpoint of the same
    name; the answers below are what Go sends (Gradio markers are already
    components).
    """

    name = "history"

    def collections(self):
        """["outputs", "favorites", "collections/<name>", ...]"""
        return self._get("collections")["collections"]

    def items(self, collection):
        """The .png paths of a collection that have a .wav next to them."""
        return self._get("items", collection=collection)["items"]

    def select(self, path):
        """What the tab shows for one item: {name: component or value}.

        Keys: bundle_name, folder_root (plain string), audio, image, json,
        npz, delete, save_to_favorites, save_to_voices, open_folder.
        """
        return self._get("select", path=path)

    def create_collection(self, name):
        """{"collections": [...], "ui": the "Created" Button}"""
        return self._remote("create-collection", name=name)

    def delete(self, directory):
        self._remote("delete", directory=directory)

    def copy(self, directory, to):
        """Copy into a collection; returns the "Saved" Button or Dropdown."""
        return self._remote("copy", directory=directory, to=to)["ui"]

    def save_to_voices(self, npz):
        """Copy an item's .npz into voices/; returns the "Saved" Button."""
        return self._remote("save-to-voices", npz=npz)["ui"]
