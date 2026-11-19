"""Bridge between server.py and the Go core's reverse proxy (stage: router).

``install()`` is a no-op unless ``TTS_CORE_URL`` is set. When it is set, the Go
core owns the public port and routes "/" to Gradio and "/<pkg>" to extensions,
so the Python proxy tree is switched off and extension routes are registered
with Go over its control API instead.

Environment (set by the Go launcher):
    TTS_CORE_URL     base URL of Go's control API, e.g. http://127.0.0.1:7771
    TTS_CORE_TOKEN   shared secret, sent as X-TTS-Core-Token
    TTS_GRADIO_PORT  port Gradio itself binds
    TTS_GRADIO_HOST  Gradio's server_name

Nothing is edited on disk: module attributes, the sys.argv is
patched in place; the config dict is never touched (see gradio_overrides), so this must run before server.py is executed and before
anything does ``from tts_webui.gradio_proxy_tree.main import add_extension_route``.
"""

import importlib
import json
import os
import sys
import time
import urllib.error
import urllib.request

__all__ = ["install", "register_route", "gradio_overrides"]

TOKEN_HEADER = "X-TTS-Core-Token"
_ATTEMPTS = 3
_RETRY_DELAY = 0.5
_TIMEOUT = 10

_installed = False

# The core is always local: never go through a system or environment proxy.
_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def _log(message):
    print(f"[core_router_bridge] {message}", flush=True)


def register_route(package_name, port, environ=None):
    """POST the route to Go. Returns True on success; never raises."""
    environ = os.environ if environ is None else environ
    base = environ.get("TTS_CORE_URL", "").rstrip("/")
    if not base:
        _log(f"TTS_CORE_URL is not set; cannot route /{package_name}")
        return False
    body = json.dumps({"prefix": f"/{package_name}", "port": int(port)}).encode()
    headers = {"Content-Type": "application/json"}
    token = environ.get("TTS_CORE_TOKEN", "")
    if token:
        headers[TOKEN_HEADER] = token
    last = None
    for attempt in range(_ATTEMPTS):
        try:
            req = urllib.request.Request(
                f"{base}/api/routes", data=body, headers=headers, method="POST"
            )
            with _OPENER.open(req, timeout=_TIMEOUT) as resp:
                resp.read()
            return True
        except urllib.error.HTTPError as e:
            last = f"HTTP {e.code} {e.reason}"
            if e.code in (400, 403, 415):  # retrying cannot help
                break
        except Exception as e:  # URLError, timeout, bad port, ...
            last = repr(e)
        if attempt + 1 < _ATTEMPTS:
            time.sleep(_RETRY_DELAY)
    _log(
        f"could not register route /{package_name} -> port {port} with the core "
        f"at {base}: {last}. The extension is running but will not be reachable "
        f"through the public port."
    )
    return False


def install():
    """Patch server.py's environment for the Go core. No-op without TTS_CORE_URL."""
    global _installed
    if not os.environ.get("TTS_CORE_URL"):
        return False
    if _installed:
        return True

    # 1. Iframes use relative URLs; must be set before main.py is imported.
    os.environ["GRADIO_TREE_URL"] = ""

    _apply_argv(sys.argv)

    main = importlib.import_module("tts_webui.gradio_proxy_tree.main")
    main.setup_gradio_proxy_tree = _setup_gradio_proxy_tree
    main.add_extension_route = add_extension_route
    _installed = True
    _log(f"Go core at {os.environ['TTS_CORE_URL']} routes the public port; Python proxy tree off")
    return True


def gradio_overrides(environ=None):
    """Private host/port for Gradio's ``launch()``, from the Go core's environment.

    These are deliberately not written into ``config["gradio_interface_options"]``:
    the Settings tab saves that dict to config.json, and the private port must
    never become the public one. Callers pass them to launch() on a copy.
    """
    environ = os.environ if environ is None else environ
    overrides = {"inbrowser": False}  # the Go launcher opens the browser
    port = environ.get("TTS_GRADIO_PORT")
    if port:
        overrides["server_port"] = int(port)
    host = environ.get("TTS_GRADIO_HOST")
    if host:
        overrides["server_name"] = host
    return overrides


def _apply_argv(argv):
    # Edit the list in place: server.py keeps a reference to sys.argv.
    if "--no-tree-proxy" not in argv:
        argv.append("--no-tree-proxy")
    # Go applies docker host/ports via the environment instead.
    argv[:] = [a for a in argv if a != "--docker"]


def _setup_gradio_proxy_tree(gr_options):
    """Replaces main.setup_gradio_proxy_tree: Go routes "/" itself."""


def add_extension_route(package_name, port):
    """Replaces main.add_extension_route: ask Go to route /<package_name>."""
    register_route(package_name, port)
