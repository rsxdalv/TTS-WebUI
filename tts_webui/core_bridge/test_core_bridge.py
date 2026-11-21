"""unittest for the dynamic bridge against a stub core API.

Run from the repo root with the project python:
    python -m unittest tts_webui.core_bridge.test_core_bridge
"""

import json
import os
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

import gradio as gr

from tts_webui.core_bridge import CoreBridgeError, bridge


class Stub:
    def __init__(self):
        self.requests = []
        stub = self

        class H(BaseHTTPRequestHandler):
            def do_POST(self):
                n = int(self.headers.get("Content-Length", 0))
                body = json.loads(self.rfile.read(n)) if n else None
                stub.requests.append(
                    (self.path, self.headers.get("X-TTS-Core-Token"), body)
                )
                if self.headers.get("X-TTS-Core-Token") != "tok":
                    code, out = 403, {"error": "forbidden"}
                elif self.path == "/api/history/copy":
                    kind = "Button" if body["to"] == "favorites" else "Dropdown"
                    code, out = 200, {
                        "path": body["to"] + "/" + body["dir"].split("/")[-1],
                        "ui": {"__gradio__": kind, "props": {"value": "Saved"}},
                    }
                else:
                    code, out = 404, {"error": "not found"}
                self.send_response(code)
                self.end_headers()
                self.wfile.write(json.dumps(out).encode())

            def log_message(self, *a):
                pass

        self.server = HTTPServer(("127.0.0.1", 0), H)
        self.url = "http://127.0.0.1:%d" % self.server.server_port
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class RoutingTest(unittest.TestCase):
    def test_known_and_unknown_names(self):
        self.assertIs(bridge.history, bridge.history)
        self.assertIn("history", dir(bridge))
        with self.assertRaises(AttributeError) as cm:
            bridge.nope
        self.assertIn("history", str(cm.exception))
        with self.assertRaises(AttributeError):
            bridge._private


class ResolveTest(unittest.TestCase):
    def test_markers(self):
        from tts_webui.core_bridge.gradio_resolve import (
            GradioMarkerError,
            marker,
            resolve,
        )

        b = resolve(marker("Button", value="Saved"))
        self.assertIsInstance(b, gr.Button)
        self.assertEqual(b.value, "Saved")
        got = resolve({"a": [marker("Dropdown", value="x", choices=["x"])], "n": 1})
        self.assertIsInstance(got["a"][0], gr.Dropdown)
        self.assertEqual(got["n"], 1)
        for bad in (
            marker("Blocks"),
            marker("Button", fn="x"),
            {"__gradio__": "Button", "props": {}, "extra": 1},
            {"__gradio__": "Button", "props": []},
        ):
            with self.assertRaises(GradioMarkerError):
                resolve(bad)


class RemoteTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_copy_goes_to_go(self):
        got = bridge.history.copy("outputs\\x", "favorites")
        self.assertIsInstance(got, gr.Button)
        self.assertEqual(got.value, "Saved")
        self.assertEqual(
            self.stub.requests,
            [("/api/history/copy", "tok", {"dir": "outputs/x", "to": "favorites"})],
        )

    def test_error_and_unreachable_raise(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError) as cm:
                bridge.history.copy("outputs/x", "favorites")
            self.assertIn("forbidden", str(cm.exception))
        self.stub.close()
        with self.assertRaises(CoreBridgeError) as cm:
            bridge.history.copy("outputs/x", "favorites")
        self.assertIn("cannot reach the TTS core", str(cm.exception))


class LocalTest(unittest.TestCase):
    def test_copy_without_core_is_copytree(self):
        env = {k: v for k, v in os.environ.items() if k != "TTS_CORE_URL"}
        old = os.getcwd()
        with tempfile.TemporaryDirectory() as d, mock.patch.dict(
            os.environ, env, clear=True
        ):
            os.chdir(d)
            try:
                os.makedirs("outputs/a")
                os.makedirs("favorites")
                open("outputs/a/a.wav", "w").close()
                self.assertFalse(bridge.history.enabled)
                got = bridge.history.copy("outputs/a", "favorites")
                self.assertIsInstance(got, gr.Button)
                self.assertTrue(os.path.exists("favorites/a/a.wav"))
                with self.assertRaises(FileExistsError):
                    bridge.history.copy("outputs/a", "favorites")
            finally:
                os.chdir(old)


if __name__ == "__main__":
    unittest.main()
