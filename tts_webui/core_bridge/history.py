import os
import shutil

from . import transport
from .base import SubBridge


class HistoryBridge(SubBridge):
    """Gallery History: ``/api/history/*`` in core/internal/serverpy/history."""

    name = "history"

    def copy(self, directory, to):
        """Copy a generation directory into a collection; returns the new path.

        Without the core this is the original handler's shutil.copytree.
        """
        if not self.enabled:
            target = os.path.join(to, os.path.basename(directory))
            shutil.copytree(directory, target)
            return target
        res = self._remote(
            "copy", {"dir": transport.wire(directory), "to": transport.wire(to)}
        )
        return transport.local(res["path"])
