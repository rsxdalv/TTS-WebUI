"""
Core proxy logic: HTTP and WebSocket forwarding for a single route.
"""

import asyncio
import logging

import httpx
import websockets
from starlette.requests import Request
from starlette.responses import Response, StreamingResponse
from starlette.websockets import WebSocket, WebSocketDisconnect

logger = logging.getLogger("gradio_proxy_tree")

# Hop-by-hop headers that must not be forwarded
_HOP_BY_HOP = frozenset(
    {
        "connection",
        "keep-alive",
        "proxy-authenticate",
        "proxy-authorization",
        "te",
        "trailers",
        "transfer-encoding",
        "upgrade",
        "content-encoding",
        "content-length",
    }
)


def _proxy_headers(
    raw_headers: list[tuple[bytes, bytes]], scheme: str
) -> dict[str, str]:
    """Build forwarding headers matching nginx proxy_set_header directives."""
    headers_dict = {k.decode("latin-1"): v.decode("latin-1") for k, v in raw_headers}
    host = headers_dict.get("host", "localhost")

    out = {}
    for key, value in headers_dict.items():
        if key.lower() not in _HOP_BY_HOP and key.lower() != "host":
            out[key] = value
    out["host"] = host
    out["x-forwarded-host"] = host
    out["x-forwarded-proto"] = scheme
    return out


async def proxy_http(scope, receive, send, *, prefix: str, target: str):
    """Forward an HTTP request, stripping *prefix* and sending to *target*."""
    request = Request(scope, receive)

    # Use raw_path to preserve special chars like file=C:/...
    raw_path = scope.get("raw_path", scope["path"].encode()).decode("latin-1")
    upstream_path = raw_path.removeprefix(prefix)
    if not upstream_path.startswith("/"):
        upstream_path = "/" + upstream_path

    query = scope.get("query_string", b"").decode("latin-1")
    url = target + upstream_path + (f"?{query}" if query else "")

    headers = _proxy_headers(scope.get("headers", []), scope.get("scheme", "http"))
    body = await request.body()

    logger.info("%s %s -> %s", scope["method"], raw_path, url)

    client = httpx.AsyncClient(follow_redirects=False, timeout=httpx.Timeout(300.0))
    try:
        req = client.build_request(
            method=scope["method"], url=url, headers=headers, content=body
        )
        upstream_resp = await client.send(req, stream=True)

        resp_headers = {
            k: v
            for k, v in upstream_resp.headers.items()
            if k.lower() not in _HOP_BY_HOP
        }

        async def stream_body():
            try:
                async for chunk in upstream_resp.aiter_bytes(chunk_size=65536):
                    yield chunk
            finally:
                await upstream_resp.aclose()
                await client.aclose()

        response = StreamingResponse(
            stream_body(), status_code=upstream_resp.status_code, headers=resp_headers
        )
        await response(scope, receive, send)

    except httpx.ConnectError as exc:
        await client.aclose()
        logger.error("Upstream %s unavailable: %s", target, exc)
        await Response("Proxy error: upstream unavailable", status_code=502)(
            scope, receive, send
        )
    except Exception as exc:
        await client.aclose()
        logger.error("Proxy error for %s: %s", target, exc)
        await Response("Proxy error: internal error", status_code=500)(
            scope, receive, send
        )


async def proxy_ws(scope, receive, send, *, prefix: str, target: str):
    """Forward a WebSocket connection, stripping *prefix* and sending to *target*."""
    websocket = WebSocket(scope, receive, send)

    raw_path = scope.get("raw_path", scope["path"].encode()).decode("latin-1")
    upstream_path = raw_path.removeprefix(prefix)
    if not upstream_path.startswith("/"):
        upstream_path = "/" + upstream_path

    query = scope.get("query_string", b"").decode("latin-1")
    ws_target = target.replace("http://", "ws://").replace("https://", "wss://")
    ws_url = ws_target + upstream_path + (f"?{query}" if query else "")

    headers_dict = {
        k.decode("latin-1"): v.decode("latin-1") for k, v in scope.get("headers", [])
    }
    host = headers_dict.get("host", "localhost")

    logger.info("WS %s -> %s", raw_path, ws_url)
    await websocket.accept()

    try:
        async with websockets.connect(
            ws_url,
            additional_headers={
                "Host": host,
                "X-Forwarded-Host": host,
                "X-Forwarded-Proto": scope.get("scheme", "ws"),
            },
            open_timeout=30,
            close_timeout=10,
        ) as upstream:

            async def client_to_upstream():
                try:
                    while True:
                        data = await websocket.receive()
                        if "text" in data and data["text"] is not None:
                            await upstream.send(data["text"])
                        elif "bytes" in data and data["bytes"] is not None:
                            await upstream.send(data["bytes"])
                        else:
                            return
                except (WebSocketDisconnect, Exception):
                    return

            async def upstream_to_client():
                try:
                    async for msg in upstream:
                        if isinstance(msg, str):
                            await websocket.send_text(msg)
                        elif isinstance(msg, bytes):
                            await websocket.send_bytes(msg)
                except (websockets.ConnectionClosed, Exception):
                    return

            done, pending = await asyncio.wait(
                [
                    asyncio.create_task(client_to_upstream()),
                    asyncio.create_task(upstream_to_client()),
                ],
                return_when=asyncio.FIRST_COMPLETED,
            )
            for task in pending:
                task.cancel()

    except Exception as exc:
        logger.error("WebSocket proxy error for %s: %s", target, exc)
    finally:
        try:
            await websocket.close()
        except Exception:
            pass
