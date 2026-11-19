"""Bridge that lets the Go core run proxy-extension processes.

``install()`` is a no-op unless ``TTS_CORE_URL`` is set. When it is set, the
module-global ``subprocess`` of
``tts_webui.extensions_loader.setup_proxy_extension`` is replaced by a shim
whose ``Popen`` asks Go (``POST /api/extensions/<pkg>/start``) to start the
extension instead of spawning it from Python. ``terminate()`` calls the stop
endpoint. Nothing else in that module changes.

The module is patched when it is first imported (an import hook), not by
importing it from ``install()``: importing it pulls in gradio and the proxy
tree, and doing that before server.py's ``dotenv_init.init()`` would make
gradio / huggingface_hub read their environment before ``.env`` is loaded.
"""

import importlib.abc
import json
import os
import subprocess as _real_subprocess
import sys
import time
import urllib.error
import urllib.request

TARGET_MODULE = "tts_webui.extensions_loader.setup_proxy_extension"

_PKG_VAR = "TTS_WEBUI_EXTENSION_PACKAGE"
_PORT_VAR = "GRADIO_SERVER_PORT"
# Set by Go itself (or sent as dedicated fields), never forwarded as "env".
_HANDLED_VARS = {_PORT_VAR, "GRADIO_ROOT_PATH", _PKG_VAR, "OPENAI_PROXY_HOST"}
_NEVER_FORWARD = {"TTS_CORE_TOKEN", "TTS_CORE_URL"}

START_TIMEOUT = 20
STOP_TIMEOUT = 30

_state = {}  # url, token, env0, port_map, finder


class CoreError(RuntimeError):
    """Go's control API could not be reached or refused the request."""


def _report(message):
    print(f"[tts-core] {message}", file=sys.stderr, flush=True)


def _user_error(message):
    """Log message and return an exception that shows it in the Gradio UI."""
    _report(message)
    gr = sys.modules.get("gradio")
    error_cls = getattr(gr, "Error", None)
    if error_cls is not None:
        try:
            return error_cls(message)
        except Exception:
            pass
    return CoreError(message)


class _Client:
    def __init__(self, url, token):
        self.url = url.rstrip("/")
        self.token = token
        # Never go through HTTP(S)_PROXY for the loopback control API.
        self._opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def call(self, method, path, body=None, timeout=START_TIMEOUT):
        """Returns (http_status, parsed_json). Raises CoreError on transport errors."""
        data = None
        headers = {"X-TTS-Core-Token": self.token}
        if method == "POST":
            data = json.dumps(body if body is not None else {}).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(
            self.url + path, data=data, headers=headers, method=method
        )
        try:
            with self._opener.open(req, timeout=timeout) as resp:
                status, raw = resp.status, resp.read()
        except urllib.error.HTTPError as e:
            status, raw = e.code, e.read()
        except (urllib.error.URLError, OSError) as e:
            reason = getattr(e, "reason", e)
            raise CoreError(
                f"cannot reach the TTS core at {self.url} ({reason}); "
                "is the launcher still running?"
            ) from e
        try:
            return status, json.loads(raw.decode() or "null")
        except ValueError:
            return status, {"error": raw.decode(errors="replace")[:200]}


def _api_error(what, status, data):
    detail = data.get("error") if isinstance(data, dict) else data
    return f"{what} failed: TTS core answered HTTP {status}: {detail}"


class GoProcess:
    """The slice of ``subprocess.Popen`` setup_proxy_extension uses."""

    def __init__(self, client, package, args, pid, port):
        self._client = client
        self.package = package
        self.args = args
        self.pid = pid
        self.port = port
        self.returncode = None

    def poll(self):
        if self.returncode is not None:
            return self.returncode
        try:
            status, data = self._client.call("GET", "/api/extensions", timeout=5)
        except CoreError:
            return None  # unknown: assume still running
        if status != 200 or not isinstance(data, list):
            return None
        for item in data:
            if item.get("package") == self.package:
                if item.get("pid") != self.pid:
                    # restarted since: the process we started is gone
                    self.returncode = 0
                elif item.get("state") in ("starting", "ready"):
                    return None
                else:
                    code = item.get("exit_code")
                    if code is None:
                        code = 0 if item.get("state") == "exited" else 1
                    self.returncode = code
                return self.returncode
        self.returncode = 0
        return self.returncode

    def wait(self, timeout=None):
        deadline = None if timeout is None else time.monotonic() + timeout
        while True:
            code = self.poll()
            if code is not None:
                return code
            if deadline is not None and time.monotonic() >= deadline:
                raise _real_subprocess.TimeoutExpired(self.args, timeout)
            time.sleep(0.25)

    def terminate(self):
        try:
            status, data = self._client.call(
                "POST", f"/api/extensions/{self.package}/stop", {}, STOP_TIMEOUT
            )
        except CoreError as e:
            raise _user_error(f"Could not stop extension {self.package}: {e}") from e
        if status not in (200, 404):  # 404: already gone, which is fine
            raise _user_error(_api_error(f"Stopping extension {self.package}", status, data))
        if self.returncode is None:
            self.returncode = 0

    kill = terminate

    def send_signal(self, _sig):
        self.terminate()


def _delta_env(env):
    env0 = _state.get("env0", {})
    return {
        k: v
        for k, v in env.items()
        if k not in _HANDLED_VARS and k not in _NEVER_FORWARD and env0.get(k) != v
    }


def _popen(args, *popen_args, env=None, **kwargs):
    client = _state["client"]
    env = os.environ if env is None else env
    package = env.get(_PKG_VAR)
    requested = env.get(_PORT_VAR)
    if not package or not requested:  # not an extension launch: behave as before
        return _real_subprocess.Popen(args, *popen_args, env=env, **kwargs)

    body = {"env": _delta_env(env)}
    try:
        body["port"] = int(requested)
    except ValueError:
        pass
    if env.get("OPENAI_PROXY_HOST"):
        body["openai_proxy_host"] = env["OPENAI_PROXY_HOST"]
    try:
        status, data = client.call(
            "POST", f"/api/extensions/{package}/start", body, START_TIMEOUT
        )
    except CoreError as e:
        raise _user_error(f"Could not start extension {package}: {e}") from e
    if status != 200 or not isinstance(data, dict) or "port" not in data:
        raise _user_error(_api_error(f"Starting extension {package}", status, data))

    actual = int(data["port"])
    if body.get("port") is not None and actual != body["port"]:
        _report(
            f"extension {package} runs on port {actual} instead of {body['port']}"
            + (" (already running)" if data.get("already_running") else "")
        )
        _state["port_map"][body["port"]] = actual
    return GoProcess(client, package, args, int(data.get("pid", 0)), actual)


class _SubprocessShim:
    """Stands in for the ``subprocess`` module global; only Popen differs."""

    Popen = staticmethod(_popen)

    def __getattr__(self, name):
        return getattr(_real_subprocess, name)


def _patch_module(mod):
    """Replace ``subprocess`` in the setup_proxy_extension module.

    If Go had to use another port than Python asked for, wait_for_server and
    add_extension_route (both called with Python's port) are given Go's.
    """
    mod.subprocess = _SubprocessShim()

    def translating(fn):
        def wrapper(port, *a, **kw):
            return fn(_state["port_map"].get(port, port), *a, **kw)

        wrapper.__wrapped__ = fn
        return wrapper

    for name in ("wait_for_server",):
        fn = getattr(mod, name, None)
        if callable(fn):
            setattr(mod, name, translating(fn))
    fn = getattr(mod, "add_extension_route", None)
    if callable(fn):

        def add_route(package_name, port, *a, **kw):
            return fn(package_name, _state["port_map"].get(port, port), *a, **kw)

        mod.add_extension_route = add_route


class _PatchLoader(importlib.abc.Loader):
    def __init__(self, inner, patch):
        self._inner = inner
        self._patch = patch

    def create_module(self, spec):
        return self._inner.create_module(spec)

    def exec_module(self, module):
        self._inner.exec_module(module)
        self._patch(module)

    def __getattr__(self, name):
        return getattr(self._inner, name)


class _PatchFinder(importlib.abc.MetaPathFinder):
    """Patches ``target`` right after it is first imported."""

    def __init__(self, target, patch):
        self.target = target
        self.patch = patch

    def find_spec(self, fullname, path=None, target=None):
        if fullname != self.target:
            return None
        for finder in sys.meta_path:
            if finder is self or not hasattr(finder, "find_spec"):
                continue
            spec = finder.find_spec(fullname, path, target)
            if spec is not None and spec.loader is not None:
                spec.loader = _PatchLoader(spec.loader, self.patch)
                if self in sys.meta_path:
                    sys.meta_path.remove(self)
                return spec
        return None


def _install_patch(target, patch):
    """Patch ``target`` now if imported, else when it is first imported."""
    mod = sys.modules.get(target)
    if mod is not None:
        patch(mod)
        return None
    finder = _PatchFinder(target, patch)
    sys.meta_path.insert(0, finder)
    return finder


def install(environ=None):
    """Route proxy-extension spawning through Go. Returns True if installed.

    No-op (returns False) unless TTS_CORE_URL is set. Safe to call twice.
    """
    environ = os.environ if environ is None else environ
    url = environ.get("TTS_CORE_URL")
    if not url:
        return False
    if _state.get("client"):
        return True
    token = environ.get("TTS_CORE_TOKEN", "")
    if not token:
        _report("TTS_CORE_URL is set but TTS_CORE_TOKEN is not; the core will refuse requests")
    _state.update(
        client=_Client(url, token),
        env0=dict(os.environ),
        port_map={},
    )
    _state["finder"] = _install_patch(TARGET_MODULE, _patch_module)
    return True


def uninstall():
    """Undo install()'s hook and state (for tests; a patched module stays patched)."""
    finder = _state.get("finder")
    if finder is not None and finder in sys.meta_path:
        sys.meta_path.remove(finder)
    _state.clear()
