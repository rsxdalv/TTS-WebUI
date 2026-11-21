"""HTTP transport to the Go core's control API, shared by every sub-bridge.

The core is reached through ``TTS_CORE_URL`` with the ``X-TTS-Core-Token``
header; both are set by the launcher when the server.py port is on. Errors are
raised, never turned into a fallback: the caller decides what "core is off"
means (see ``enabled``).
"""

import json
import os
import urllib.error
import urllib.parse
import urllib.request

TOKEN_HEADER = "X-TTS-Core-Token"
DEFAULT_TIMEOUT = 120

# The core is always local: never go through a system or environment proxy.
_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


class CoreBridgeError(RuntimeError):
    """The Go core could not be reached or refused the request."""


def enabled():
    return bool(os.environ.get("TTS_CORE_URL"))


def wire(path):
    """A path as sent to Go: forward slashes (Go accepts both)."""
    return str(path).replace("\\", "/")


def local(path):
    """A path from Go in the form os.path.join gave Gradio before."""
    return path.replace("/", os.sep)


def call(method, path, query=None, body=None, timeout=DEFAULT_TIMEOUT):
    url = os.environ["TTS_CORE_URL"].rstrip("/") + path
    if query:
        url += "?" + urllib.parse.urlencode(query)
    headers = {TOKEN_HEADER: os.environ.get("TTS_CORE_TOKEN", "")}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with _OPENER.open(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode(errors="replace")
        try:
            detail = json.loads(raw).get("error", raw)
        except ValueError:
            detail = raw
        raise CoreBridgeError(f"{path}: {detail[:300]} (HTTP {e.code})") from e
    except (urllib.error.URLError, OSError) as e:
        raise CoreBridgeError(
            f"cannot reach the TTS core at {url} ({getattr(e, 'reason', e)}); "
            "is the launcher still running?"
        ) from e
