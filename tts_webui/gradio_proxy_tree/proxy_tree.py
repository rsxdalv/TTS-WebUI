"""
GradioProxyTree - A dynamic reverse proxy server for multiple Gradio apps.

Each route maps a URL prefix to a local port, e.g.:
    /mms  -> http://127.0.0.1:20001
    /bark -> http://127.0.0.1:20002

Routes can be added at any time (even while the server is running).
"""

import logging
import threading
from typing import Optional

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from starlette.requests import Request
from starlette.types import ASGIApp, Receive, Scope, Send

from .handlers import proxy_http, proxy_ws

logger = logging.getLogger("gradio_proxy_tree")


class _RouteDispatcher:
    """ASGI middleware that dispatches requests to the right upstream by prefix."""

    def __init__(self, app: ASGIApp):
        self.app = app
        # {"/mms": "http://127.0.0.1:20001", ...}
        self._routes: dict[str, str] = {}
        self._lock = threading.Lock()

    def add_route(self, prefix: str, target: str):
        with self._lock:
            self._routes[prefix] = target
        logger.info("Route added: %s -> %s", prefix, target)

    @property
    def routes(self) -> dict[str, str]:
        with self._lock:
            return dict(self._routes)

    def _match(self, path: str) -> Optional[tuple[str, str]]:
        """Find the longest matching prefix for a path."""
        with self._lock:
            best = None
            for prefix, target in self._routes.items():
                if path == prefix or path.startswith(prefix + "/"):
                    if best is None or len(prefix) > len(best[0]):
                        best = (prefix, target)
            return best

    async def __call__(self, scope: Scope, receive: Receive, send: Send):
        if scope["type"] not in ("http", "websocket"):
            await self.app(scope, receive, send)
            return

        path = scope["path"]
        match = self._match(path)

        if match is None:
            await self.app(scope, receive, send)
            return

        prefix, target = match
        if scope["type"] == "http":
            await proxy_http(scope, receive, send, prefix=prefix, target=target)
        else:
            await proxy_ws(scope, receive, send, prefix=prefix, target=target)


class GradioProxyTree:
    """
    Dynamic reverse proxy that can serve multiple Gradio apps under one port.

    Example::

        tree = GradioProxyTree(port=8079)
        tree.add_route("/mms", 20001)
        tree.add_route("/bark", 20002)
        tree.start()  # blocking
    """

    def __init__(self, host: str = "0.0.0.0", port: int = 8079):
        self.host = host
        self.port = port

        self._app = FastAPI(title="Gradio Proxy Tree")
        self._dispatcher = _RouteDispatcher(self._app)

        self._register_management_routes()

        self._server: Optional[uvicorn.Server] = None
        self._thread: Optional[threading.Thread] = None

    def _register_management_routes(self):
        @self._app.get("/healthz")
        async def health():
            return {"status": "ok", "routes": self._dispatcher.routes}

        @self._app.get("/routes")
        async def list_routes():
            return {"routes": self._dispatcher.routes}

        @self._app.api_route(
            "/{path:path}",
            methods=["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"],
        )
        async def catch_all(request: Request, path: str = ""):
            return JSONResponse(
                {"detail": f"Not Found (proxy server - no route matches /{path})", "proxy": True},
                status_code=404,
            )

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def add_route(self, prefix: str, port: int, host: str = "127.0.0.1"):
        """
        Add a proxy route: requests to *prefix* are forwarded to *host:port*.

        Can be called before or after the server is started.
        """
        if not prefix.startswith("/"):
            prefix = "/" + prefix
        prefix = prefix.rstrip("/")
        target = f"http://{host}:{port}"
        self._dispatcher.add_route(prefix, target)

    @property
    def routes(self) -> dict[str, str]:
        return self._dispatcher.routes

    def start(self):
        """Start the proxy server (blocking)."""
        logger.info("Proxy tree listening on %s:%d", self.host, self.port)
        for prefix, target in self._dispatcher.routes.items():
            logger.info("  %s -> %s", prefix, target)

        config = uvicorn.Config(self._dispatcher, host=self.host, port=self.port, log_level="info")
        self._server = uvicorn.Server(config)
        self._server.run()

    def start_background(self) -> threading.Thread:
        """Start the proxy server in a background thread. Returns the thread."""
        self._thread = threading.Thread(target=self.start, daemon=True, name="proxy-tree")
        self._thread.start()
        logger.info("Proxy tree started in background thread")
        return self._thread

    def stop(self):
        """Signal the server to shut down."""
        if self._server:
            self._server.should_exit = True
