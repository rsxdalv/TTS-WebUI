import os
import shutil

from . import transport
from .base import SubBridge
from .gradio_resolve import marker, resolve


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history."""

    name = "history"

    def copy(self, directory, to):
        """Copy a generation directory into a collection.

        Returns the Gradio update for the control that triggered it: the
        "Saved" Button for favorites, the "Saved" Dropdown for a collection.
        With the core, Go copies and sends the update as a marker; without it
        the copy is the original handler's shutil.copytree.
        """
        if not self.enabled:
            shutil.copytree(directory, os.path.join(to, os.path.basename(directory)))
            kind = "Button" if to == "favorites" else "Dropdown"
            return resolve(marker(kind, value="Saved"))
        res = self._remote(
            "copy", {"dir": transport.wire(directory), "to": transport.wire(to)}
        )
        return resolve(res["ui"])
