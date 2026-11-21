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
STREAM_TIMEOUT = 3600

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


def _open(method, path, query, body, timeout):
    """Open a request to the core; returns the response (a context manager)."""
    if not enabled():
        raise CoreBridgeError(
            "the Go core is not running (TTS_CORE_URL is not set); start the app "
            "through the launcher with TTS_GO_SERVERPY=1"
        )
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
        return _OPENER.open(req, timeout=timeout)
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


def call(method, path, query=None, body=None, timeout=DEFAULT_TIMEOUT):
    with _open(method, path, query, body, timeout) as resp:
        return json.loads(resp.read().decode())


def stream(path, body=None, timeout=STREAM_TIMEOUT):
    """POST and yield the answer's JSON lines (NDJSON) as they arrive.

    ``timeout`` is how long a single read may block: an install can be quiet
    for minutes between lines.
    """
    with _open("POST", path, None, body if body is not None else {}, timeout) as resp:
        try:
            for raw in resp:
                raw = raw.strip()
                if raw:
                    yield json.loads(raw.decode())
        except OSError as e:
            raise CoreBridgeError(f"{path}: the stream broke ({e})") from e
