"""Bridge that lets the Gallery History tab use the Go core for its file work.

``enabled()`` is true only when ``TTS_CORE_URL`` is set (the launcher sets it
when the server.py port is on). The Gradio handlers check it and, when it is
true, call these functions instead of touching the disk themselves; when it is
false they run the original Python code unchanged.

If Go cannot be reached the error is raised, never a fallback to Python: Go is
the one place that validates paths before deleting or copying.

Paths are sent with ``/``; Go accepts both separators. Paths coming back are
converted to ``os.sep`` so Gradio sees the same form ``glob`` and
``os.path.join`` gave it before.
"""

import json
import os
import urllib.error
import urllib.parse
import urllib.request

__all__ = [
    "CoreHistoryError",
    "enabled",
    "collections",
    "items",
    "item_json",
    "create_collection",
    "delete",
]

TOKEN_HEADER = "X-TTS-Core-Token"
_TIMEOUT = 120  # copying a few audio files; everything else is instant

# The core is always local: never go through a system or environment proxy.
_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


class CoreHistoryError(RuntimeError):
    """Go's history API could not be reached or refused the request."""


def enabled():
    return bool(os.environ.get("TTS_CORE_URL"))


def _local(path):
    return path.replace("/", os.sep)


def _wire(path):
    return str(path).replace("\\", "/")


def _call(method, op, query=None, body=None):
    url = os.environ["TTS_CORE_URL"].rstrip("/") + "/api/history/" + op
    if query:
        url += "?" + urllib.parse.urlencode(query)
    headers = {TOKEN_HEADER: os.environ.get("TTS_CORE_TOKEN", "")}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with _OPENER.open(req, timeout=_TIMEOUT) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode(errors="replace")
        try:
            detail = json.loads(raw).get("error", raw)
        except ValueError:
            detail = raw
        raise CoreHistoryError(f"{op}: {detail[:300]} (HTTP {e.code})") from e
    except (urllib.error.URLError, OSError) as e:
        raise CoreHistoryError(
            f"cannot reach the TTS core at {url} ({getattr(e, 'reason', e)}); "
            "is the launcher still running?"
        ) from e


def collections():
    """Same list as get_collections(): outputs, favorites, collections/<name>."""
    return [_local(c) for c in _call("GET", "collections")["collections"]]


def items(collection):
    """The .png paths of a collection that have a .wav next to them."""
    return [_local(p) for p in _call("GET", "items", {"collection": _wire(collection)})["items"]]


def item_json(path):
    """The metadata JSON stored next to a .wav (or .png)."""
    return _call("GET", "item", {"path": _wire(path)})["json"]


def create_collection(name):
    """Create collections/<name>; returns the new collection list."""
    res = _call("POST", "create-collection", body={"name": name})
    return [_local(c) for c in res["collections"]]


def delete(directory):
    _call("POST", "delete", body={"directory": _wire(directory)})

