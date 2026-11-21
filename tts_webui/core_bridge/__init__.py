"""One entry point to the Go core for the Python side.

    from tts_webui.core_bridge import bridge
    bridge.history.copy(directory, "favorites")

``bridge.<name>`` returns the sub-bridge registered for ``<name>``. The
registry is a hand-written table, not discovery: add a class in this package
and one line below. Without ``TTS_CORE_URL`` (the Go core) every call raises
CoreBridgeError.
"""

from .base import SubBridge
from .config import ConfigBridge
from .extcatalog import ExtCatalogBridge
from .history import HistoryBridge
from .transport import CoreBridgeError, enabled

__all__ = ["bridge", "Bridge", "SubBridge", "CoreBridgeError", "enabled"]

_SUBBRIDGES = {
    "config": ConfigBridge,
    "extcatalog": ExtCatalogBridge,
    "history": HistoryBridge,
}


class Bridge:
    def __init__(self):
        self._instances = {}

    def __getattr__(self, name):
        if name.startswith("_") or name not in _SUBBRIDGES:
            raise AttributeError(f"no core bridge named {name!r}; known: {sorted(_SUBBRIDGES)}")
        inst = self._instances.get(name)
        if inst is None:
            inst = self._instances[name] = _SUBBRIDGES[name]()
        return inst

    def __dir__(self):
        return sorted(_SUBBRIDGES)


bridge = Bridge()
