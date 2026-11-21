from .base import GET, POST, SubBridge


class LogsBridge(SubBridge):
    """Log Viewer: ``/api/logs/*`` in core/internal/serverpy/logview.

    ``file`` is a log file name inside installer_scripts/logs (never a path).
    What Go answers, per operation:
    - files: ["a.log", ...] newest first
    - pip_logs(kind): [{"name", "file"}, ...] for kind "install" / "uninstall"
    - stats, summary(kind), info(file), analyze(file): Markdown strings
    - read(file, search, max_lines): the text, or an in-band message
      ("File not found", "Log file is empty", "No matches found for '...'")
    - delete(file), delete_all: {"status": str, "files": [...]}
    """

    name = "logs"
    ops = {
        "files": (GET, ()),
        "pip_logs": (GET, ("kind",)),
        "stats": (GET, ()),
        "summary": (GET, ("kind",)),
        "info": (GET, ("file",)),
        "analyze": (GET, ("file",)),
        "read": (GET, ("file", "search", "max_lines")),
        "delete": (POST, ("file",)),
        "delete_all": (POST, ()),
    }
