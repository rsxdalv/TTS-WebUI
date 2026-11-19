"""Bridge that makes the Go core the only reader and writer of config.json.

``install()`` is a no-op unless ``TTS_CORE_URL`` is set. When it is set,
``tts_webui.config.load_config.load_config`` reads the document from Go
(``GET /api/config``) and ``tts_webui.config._save_config._save_config`` writes
it through Go (``PUT /api/config``); Go writes the file exactly like
``json.dump(data, f, indent=2)``.

Both modules only import ``json``/``os``, so ``install()`` imports them and
replaces the functions directly. It must run before ``tts_webui.config.config``
(which calls ``load_config()`` at import) and before any module that does
``from tts_webui.config._save_config import _save_config``: those bind the name
at import and would keep the original. server_core.py installs it first.

If Go cannot be reached the error is raised, never a fallback to the file:
two writers is the thing this bridge exists to prevent.
"""

import json
import os
import sys
import urllib.error
import urllib.request

__all__ = ["install", "load_config", "save_config"]

TOKEN_HEADER = "X-TTS-Core-Token"
_TIMEOUT = 10

_installed = False

# The core is always local: never go through a system or environment proxy.
_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


class CoreConfigError(RuntimeError):
    """Go's config API could not be reached or refused the request."""


def _call(method, body=None):
    url = os.environ["TTS_CORE_URL"].rstrip("/") + "/api/config"
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
        detail = e.read().decode(errors="replace")[:200]
        raise CoreConfigError(f"{method} /api/config: HTTP {e.code}: {detail}") from e
    except (urllib.error.URLError, OSError) as e:
        raise CoreConfigError(
            f"cannot reach the TTS core at {url} ({getattr(e, 'reason', e)}); "
            "is the launcher still running?"
        ) from e


def load_config():
    """Replacement for load_config.load_config: the document Go holds."""
    return _call("GET")


def save_config(data):
    """Replacement for _save_config._save_config: Go writes config.json."""
    _call("PUT", data)


def _save_json(filename, data):
    if os.path.basename(filename) == "config.json":
        save_config(data)
    else:
        with open(filename, "w") as outfile:
            json.dump(data, outfile, indent=2)


def install():
    """Patch config loading and saving. No-op without TTS_CORE_URL."""
    global _installed
    if not os.environ.get("TTS_CORE_URL"):
        return False
    if _installed:
        return True

    from tts_webui.config import _save_config as save_mod
    from tts_webui.config import load_config as load_mod

    load_mod.load_config = load_config
    save_mod._save_config = save_config
    save_mod._save_json = _save_json
    _installed = True
    print("[tts-core] config.json is owned by the Go core", file=sys.stderr, flush=True)
    return True
