import time

from . import transport
from .base import SubBridge

POLL_SECONDS = 0.25
# Go gives up on an extension that does not listen within 120 s (state failed).
READY_TIMEOUT = 130


class ExtensionsBridge(SubBridge):
    """Extension processes: ``/api/extensions`` in core/internal/serverpy/extprocs.

    Go runs the proxy extensions (a native extension in its own venv, or in the
    main env), picks their port and registers the route ``/<package>`` on the
    router once the process listens. Python only asks:

    - start(package, openai_proxy_host=""): {"port", "pid"[, "already_running"]};
      it returns when the process is spawned, not when it listens
    - wait_ready(package, timeout): returns when the route is up; raises
      CoreBridgeError with Go's reason when the process failed or exited
    - stop(package): stops it and removes the route; an extension Go does not
      know is already stopped
    - status(): [{"package", "port", "pid", "state", "error", "exit_code"}]
    """

    name = "extensions"

    def start(self, package, openai_proxy_host=""):
        body = {"openai_proxy_host": openai_proxy_host} if openai_proxy_host else {}
        return transport.call("POST", f"/api/extensions/{package}/start", body=body)

    def stop(self, package):
        try:
            transport.call("POST", f"/api/extensions/{package}/stop", body={})
        except transport.CoreBridgeError as e:
            if e.status != 404:
                raise

    def status(self):
        return transport.call("GET", "/api/extensions")

    def wait_ready(self, package, timeout=READY_TIMEOUT):
        deadline = time.monotonic() + timeout
        while True:
            state = next((s for s in self.status() if s["package"] == package), None)
            if state is None:
                raise transport.CoreBridgeError(f"extension {package} is not running")
            if state["state"] == "ready":
                return state
            if state["state"] != "starting":
                reason = state.get("error") or f"exit code {state.get('exit_code')}"
                raise transport.CoreBridgeError(
                    f"extension {package} {state['state']}: {reason}"
                )
            if time.monotonic() >= deadline:
                raise transport.CoreBridgeError(
                    f"extension {package} did not start within {timeout} s"
                )
            time.sleep(POLL_SECONDS)
