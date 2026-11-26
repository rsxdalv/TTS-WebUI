import os
import subprocess
import sys

from tts_webui.utils.get_path_from_root import get_path_from_root

# The history tabs let the client choose which folder to open, so the desktop
# opener must only ever be handed a folder of a collection (never a file: on
# Windows `explorer <file>` runs it).
COLLECTION_ROOTS = ("outputs", "favorites", "collections")


def _open_in_file_manager(folder_path: str):
    if sys.platform == "darwin":
        subprocess.check_call(["open", "--", folder_path])
    elif sys.platform.startswith("linux"):
        subprocess.check_call(["xdg-open", folder_path])
    elif sys.platform == "win32":
        subprocess.Popen(["explorer", folder_path])
    else:
        raise NotImplementedError(f"open_folder is not supported on {sys.platform}")


def open_folder(folder_path: str):
    """Open a folder chosen by the program (model folders and the like).

    The path must be an existing directory. Anything else is refused, so a
    file is never handed to the desktop opener.
    """
    if not isinstance(folder_path, str) or not folder_path:
        raise ValueError("folder path must be a non-empty string")
    real = os.path.realpath(folder_path)
    if not os.path.isdir(real):
        raise ValueError(f"not a folder: {folder_path!r}")
    _open_in_file_manager(real)


def resolve_collection_folder(rel_path: str, root: str | None = None) -> str:
    """Resolve a client-supplied path to a folder inside a collection.

    Accepts outputs/..., favorites/... and collections/<name>/... relative to
    the install root. Refuses absolute paths, drive letters, "..", dot names
    and control characters, and a path whose real location leaves its
    collection (a symlink). The top of a collection (outputs/ or favorites/)
    may itself be a link the user made.
    """
    if not isinstance(rel_path, str) or not rel_path:
        raise ValueError("folder path must be a non-empty string")
    segs = rel_path.replace("\\", "/").split("/")
    for s in segs:
        if (
            s in ("", ".", "..")
            or s.startswith(".")
            or ":" in s
            or any(ord(c) < 0x20 or ord(c) == 0x7F for c in s)
        ):
            raise ValueError(f"bad path element {s!r}")
    if segs[0] not in COLLECTION_ROOTS:
        raise ValueError("path must start with outputs, favorites or collections")
    top_len = 2 if segs[0] == "collections" else 1
    if len(segs) < top_len:
        raise ValueError("collections needs a name")

    root = os.path.realpath(root or get_path_from_root())
    top = os.path.realpath(os.path.join(root, *segs[:top_len]))
    target = os.path.realpath(os.path.join(root, *segs))
    top_n, target_n = os.path.normcase(top), os.path.normcase(target)
    if os.path.commonpath([top_n, target_n]) != top_n:
        raise ValueError("path leaves the collection")
    if not os.path.isdir(target):
        raise ValueError(f"not a folder: {rel_path!r}")
    return target


def open_collection_folder(rel_path: str, root: str | None = None):
    """open_folder for a path that came from the client."""
    _open_in_file_manager(resolve_collection_folder(rel_path, root))


if __name__ == "__main__":
    open_folder(get_path_from_root("data", "models"))
