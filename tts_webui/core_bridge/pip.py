from .base import GET, SubBridge


class PipBridge(SubBridge):
    """Installed Packages: ``/api/pip/*`` in core/internal/serverpy/piplist.

    What Go answers, per operation:
    - list: [{"name": str, "version": str}, ...] of the main env, sorted
      case-insensitively by name. A failing pip raises CoreBridgeError with
      the first lines of pip's stderr.
    """

    name = "pip"
    ops = {
        "list": (GET, ()),
    }
