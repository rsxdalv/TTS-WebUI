from . import transport
from .base import SubBridge
from .gradio_resolve import resolve


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history."""

    name = "history"

    def copy(self, directory, to):
        """Copy a generation directory into a collection (Go does the copy).

        Returns the Gradio update Go sends for the control that triggered it:
        the "Saved" Button for favorites, the "Saved" Dropdown otherwise.
        """
        res = self._remote(
            "copy", {"dir": transport.wire(directory), "to": transport.wire(to)}
        )
        return resolve(res["ui"])
