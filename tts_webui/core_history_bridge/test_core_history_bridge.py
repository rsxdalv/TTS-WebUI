"""unittest for the history bridge against a stub API, plus the switch-off rule.

Run from the repo root with the project python (needs gradio for the handlers):
    python -m unittest tts_webui.core_history_bridge.test_core_history_bridge
"""

import json
import os
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

from tts_webui import core_history_bridge as bridge


class Stub:
    def __init__(self):
        self.requests = []
        stub = self

        class H(BaseHTTPRequestHandler):
            def _reply(self, code, obj):
                self.send_response(code)
                self.end_headers()
                self.wfile.write(json.dumps(obj).encode())

            def _handle(self):
                n = int(self.headers.get("Content-Length", 0))
                body = json.loads(self.rfile.read(n)) if n else None
                stub.requests.append(
                    (self.command, self.path, self.headers.get("X-TTS-Core-Token"), body)
                )
                if self.headers.get("X-TTS-Core-Token") != "tok":
                    return self._reply(403, {"error": "forbidden"})
                p = self.path.split("?")[0]
                if p == "/api/history/collections":
                    return self._reply(200, {"collections": ["outputs", "favorites", "collections/a"]})
                if p == "/api/history/items":
                    return self._reply(200, {"items": ["outputs/x/x.png"]})
                if p == "/api/history/item":
                    return self._reply(200, {"json": {"text": "hi"}})
                if p == "/api/history/create-collection":
                    return self._reply(200, {"collections": ["outputs", "favorites", "collections/" + body["name"]]})
                if p == "/api/history/delete":
                    if body["dir"] == "gone":
                        return self._reply(404, {"error": "not found: gone"})
                    return self._reply(200, {"ok": True})
                if p == "/api/history/copy":
                    return self._reply(200, {
                        "path": body["to"] + "/" + body["dir"].split("/")[-1],
                        "ui": {"__gradio__": "Button", "props": {"value": "Saved"}},
                    })
                self._reply(404, {"error": "not found"})

            do_GET = do_POST = _handle

            def log_message(self, *a):
                pass

        self.server = HTTPServer(("127.0.0.1", 0), H)
        self.url = "http://127.0.0.1:%d" % self.server.server_port
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class BridgeTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_calls_and_path_forms(self):
        sep = os.sep
        self.assertEqual(
            bridge.collections(), ["outputs", "favorites", "collections" + sep + "a"]
        )
        self.assertEqual(bridge.items("collections" + sep + "a"), ["outputs" + sep + "x" + sep + "x.png"])
        self.assertEqual(bridge.item_json("outputs\\x\\x.wav"), {"text": "hi"})
        self.assertEqual(
            bridge.create_collection("new")[-1], "collections" + sep + "new"
        )
        bridge.delete("outputs\\x")
        self.assertEqual(
            bridge.copy("outputs\\x", "favorites"), "favorites" + sep + "x"
        )
        reqs = self.stub.requests
        self.assertTrue(all(r[2] == "tok" for r in reqs))
        self.assertIn("collection=collections%2Fa", reqs[1][1])
        self.assertIn("path=outputs%2Fx%2Fx.wav", reqs[2][1])
        self.assertEqual(reqs[3][3], {"name": "new"})
        self.assertEqual(reqs[4][3], {"dir": "outputs/x"})
        self.assertEqual(reqs[5][3], {"dir": "outputs/x", "to": "favorites"})

    def test_http_error_carries_go_message(self):
        with self.assertRaises(bridge.CoreHistoryError) as cm:
            bridge.delete("gone")
        self.assertIn("not found: gone", str(cm.exception))
        self.assertIn("404", str(cm.exception))

    def test_wrong_token(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(bridge.CoreHistoryError):
                bridge.collections()

    def test_unreachable_raises_and_does_not_fall_back(self):
        self.stub.close()
        with tempfile.TemporaryDirectory() as d, mock.patch(
            "os.getcwd", return_value=d
        ):
            with self.assertRaises(bridge.CoreHistoryError) as cm:
                bridge.collections()
            self.assertIn("cannot reach the TTS core", str(cm.exception))

    def test_handlers_use_go(self):
        from tts_webui.history_tab.collections_directories_atom import get_collections
        from tts_webui.history_tab.delete_generation_cb import delete_generation_cb
        from tts_webui.history_tab.save_to_favorites import (
            save_to_collection,
            save_to_favorites,
        )

        # importing collections_directories_atom already listed once
        self.stub.requests.clear()
        self.assertEqual(get_collections()[:2], ["outputs", "favorites"])
        save_to_favorites("outputs/x")
        save_to_collection("outputs/x", "collections/a")
        seen = []
        delete_generation_cb(lambda *a: seen.append(a) or "ok")("outputs/x", "outputs")
        self.assertEqual(seen, [("outputs",)])
        paths = [r[1].split("?")[0] for r in self.stub.requests]
        self.assertEqual(
            paths,
            ["/api/history/collections", "/api/history/copy", "/api/history/copy", "/api/history/delete"],
        )


class SwitchOffTest(unittest.TestCase):
    """Without TTS_CORE_URL the original Python code runs, on the local disk."""

    def test_python_path_unchanged(self):
        from tts_webui.history_tab.delete_generation_cb import delete_generation_cb
        from tts_webui.history_tab.save_to_favorites import (
            save_to_collection,
            save_to_favorites,
        )

        env = {k: v for k, v in os.environ.items() if k != "TTS_CORE_URL"}
        old = os.getcwd()
        with tempfile.TemporaryDirectory() as d, mock.patch.dict(
            os.environ, env, clear=True
        ):
            os.chdir(d)
            try:
                for p in ("outputs/a", "favorites", "collections/c"):
                    os.makedirs(p)
                open("outputs/a/a.wav", "w").close()
                self.assertFalse(bridge.enabled())
                save_to_favorites("outputs/a")
                save_to_collection("outputs/a", "collections/c")
                self.assertTrue(os.path.exists("favorites/a/a.wav"))
                self.assertTrue(os.path.exists("collections/c/a/a.wav"))
                delete_generation_cb(lambda *a: None)("outputs/a")
                self.assertFalse(os.path.exists("outputs/a"))
            finally:
                os.chdir(old)


if __name__ == "__main__":
    unittest.main()
