from .base import SubBridge


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history."""

    name = "history"

    def copy(self, directory, to):
        """Copy a generation directory into a collection (Go does the copy).

        Returns the Gradio update Go sends for the control that triggered it:
        the "Saved" Button for favorites, the "Saved" Dropdown otherwise.
        """
        return self._remote("copy", directory=directory, to=to)["ui"]
