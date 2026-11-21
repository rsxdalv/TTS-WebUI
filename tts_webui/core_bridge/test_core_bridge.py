"""unittest for the bridge against a stub core API (canned answers).

Run from the repo root with the project python:
    python -m unittest tts_webui.core_bridge.test_core_bridge
"""

import json
import os
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock
from urllib.parse import parse_qs, urlparse

import gradio as gr

from tts_webui.core_bridge import CoreBridgeError, bridge


def M(kind, **props):
    return {"__gradio__": kind, "props": props}


ANSWERS = {
    ("GET", "collections"): ["outputs", "favorites", "collections/a"],
    ("GET", "items"): ["outputs/x/x.png"],
    ("GET", "select"): {
        "bundle_name": M("Markdown", value="outputs/x"),
        "folder_root": "outputs/x",
        "audio": M("Audio", value="outputs/x/x.wav", label="outputs/x/x.wav"),
        "image": M("Image", value="outputs/x/x.png"),
        "json": M("JSON", value={"text": "hi"}),
        "npz": M("Textbox", value="outputs/x/x.npz"),
        "delete": M("Button", visible=True),
        "save_to_favorites": M("Button", visible=True, value="Save to favorites"),
        "save_to_voices": M("Button", visible=True, value="Save to voices"),
        "open_folder": M("Button", visible=True),
    },
    ("POST", "create-collection"): {
        "collections": ["outputs", "favorites", "collections/new"],
        "ui": M("Button", value="Created"),
    },
    ("POST", "delete"): {"ok": True},
    ("POST", "copy"): M("Button", value="Saved"),
    ("POST", "save-to-voices"): M("Button", value="Saved"),
}


class Stub:
    def __init__(self):
        self.requests = []  # (method, op, query, body, token)
        stub = self

        class H(BaseHTTPRequestHandler):
            def _handle(self):
                u = urlparse(self.path)
                n = int(self.headers.get("Content-Length", 0))
                body = json.loads(self.rfile.read(n)) if n else None
                op = u.path.removeprefix("/api/history/")
                token = self.headers.get("X-TTS-Core-Token")
                query = {k: v[0] for k, v in parse_qs(u.query).items()}
                stub.requests.append((self.command, op, query, body, token))
                if token != "tok":
                    code, out = 403, {"error": "forbidden"}
                elif op == "delete" and body["directory"] == "gone":
                    code, out = 404, {"error": "not found: gone"}
                elif (self.command, op) in ANSWERS:
                    code, out = 200, ANSWERS[(self.command, op)]
                else:
                    code, out = 404, {"error": "not found"}
                self.send_response(code)
                self.end_headers()
                self.wfile.write(json.dumps(out).encode())

            do_GET = do_POST = _handle

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

    def test_plain_values(self):
        self.assertEqual(
            bridge.history.collections(), ANSWERS[("GET", "collections")]
        )
        self.assertEqual(bridge.history.items("collections\\a"), ["outputs/x/x.png"])
        self.assertEqual(self.stub.requests[1][2], {"collection": "collections\\a"})
        self.assertEqual(bridge.history.delete(directory="outputs\\x"), {"ok": True})
        self.assertEqual(self.stub.requests[2][3], {"directory": "outputs\\x"})

    def test_select_returns_components_and_plain_values(self):
        ui = bridge.history.select(path="outputs/x/x.png")
        self.assertEqual(self.stub.requests[0][2], {"path": "outputs/x/x.png"})
        self.assertEqual(ui["folder_root"], "outputs/x")
        self.assertIsInstance(ui["bundle_name"], gr.Markdown)
        self.assertIsInstance(ui["audio"], gr.Audio)
        self.assertEqual(ui["audio"].label, "outputs/x/x.wav")
        self.assertIsInstance(ui["image"], gr.Image)
        self.assertIsInstance(ui["json"], gr.JSON)
        self.assertEqual(ui["json"].value, {"text": "hi"})
        self.assertIsInstance(ui["npz"], gr.Textbox)
        for k in ("delete", "save_to_favorites", "save_to_voices", "open_folder"):
            self.assertIsInstance(ui[k], gr.Button)

    def test_create_collection_save_copy(self):
        res = bridge.history.create_collection(name="new")
        self.assertEqual(res["collections"][-1], "collections/new")
        self.assertEqual(res["ui"].value, "Created")
        self.assertEqual(self.stub.requests[0][3], {"name": "new"})
        self.assertEqual(bridge.history.copy("outputs\\x", "favorites").value, "Saved")
        self.assertEqual(
            self.stub.requests[1][3], {"directory": "outputs\\x", "to": "favorites"}
        )
        self.assertEqual(
            bridge.history.save_to_voices(npz="outputs/x/x.npz").value, "Saved"
        )
        self.assertEqual(self.stub.requests[2][3], {"npz": "outputs/x/x.npz"})
        self.assertTrue(all(r[4] == "tok" for r in self.stub.requests))

    def test_http_error_carries_go_message(self):
        with self.assertRaises(CoreBridgeError) as cm:
            bridge.history.delete(directory="gone")
        self.assertIn("not found: gone", str(cm.exception))
        self.assertIn("404", str(cm.exception))

    def test_wrong_token_and_unreachable_raise(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError) as cm:
                bridge.history.collections()
            self.assertIn("forbidden", str(cm.exception))
        self.stub.close()
        with self.assertRaises(CoreBridgeError) as cm:
            bridge.history.collections()
        self.assertIn("cannot reach the TTS core", str(cm.exception))

    def test_handlers_go_through_the_bridge(self):
        from tts_webui.history_tab.collections_directories_atom import get_collections
        from tts_webui.history_tab.delete_generation_cb import delete_generation_cb
        from tts_webui.history_tab.save_to_favorites import (
            save_to_collection,
            save_to_favorites,
        )

        self.stub.requests.clear()  # importing the atom listed once
        self.assertEqual(get_collections()[:2], ["outputs", "favorites"])
        self.assertIsInstance(save_to_favorites("outputs/x"), gr.Button)
        self.assertIsInstance(
            save_to_collection("outputs/x", "collections/a"), gr.Button
        )
        seen = []
        delete_generation_cb(lambda *a: seen.append(a) or "ok")("outputs/x", "outputs")
        self.assertEqual(seen, [("outputs",)])
        self.assertEqual(
            [r[1] for r in self.stub.requests],
            ["collections", "copy", "copy", "delete"],
        )


class OpsTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_positional_keyword_and_argument_errors(self):
        h = bridge.history
        self.assertEqual(h.copy("outputs/x", "favorites").value, "Saved")
        self.assertEqual(h.copy(directory="outputs/x", to="favorites").value, "Saved")
        self.assertEqual(h.copy("outputs/x", to="favorites").value, "Saved")
        self.assertEqual({tuple(r[3].items()) for r in self.stub.requests}, {(("directory", "outputs/x"), ("to", "favorites"))})
        for bad in (
            lambda: h.copy("a"),
            lambda: h.copy("a", "b", "c"),
            lambda: h.copy("a", directory="b", to="c"),
            lambda: h.copy(directory="a", nope="b"),
            lambda: h.collections("x"),
        ):
            with self.assertRaises(TypeError):
                bad()
        with self.assertRaises(AttributeError) as cm:
            h.nope
        self.assertIn("copy", str(cm.exception))
        self.assertEqual(len(self.stub.requests), 3)  # bad calls never reached Go

    def test_bound_operation_is_a_gradio_handler(self):
        handler = bridge.history.save_to_voices
        self.assertEqual(handler.__name__, "save_to_voices")
        with gr.Blocks():
            txt = gr.Textbox(visible=False)
            btn = gr.Button()
            btn.click(fn=handler, inputs=txt, outputs=btn)


class CoreOffTest(unittest.TestCase):
    def test_every_call_raises_a_clear_error_without_core(self):
        env = {k: v for k, v in os.environ.items() if k != "TTS_CORE_URL"}
        with mock.patch.dict(os.environ, env, clear=True):
            for call in (
                lambda: bridge.history.collections(),
                lambda: bridge.history.select(path="outputs/x/x.png"),
                lambda: bridge.history.delete(directory="outputs/x"),
                lambda: bridge.history.copy("outputs/x", "favorites"),
            ):
                with self.assertRaises(CoreBridgeError) as cm:
                    call()
                self.assertIn("not running", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
