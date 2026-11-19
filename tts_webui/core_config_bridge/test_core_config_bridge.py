"""unittest for the config bridge against a stub config API.

Run from the repo root:  python -m unittest tts_webui.core_config_bridge.test_core_config_bridge
"""

import importlib
import json
import os
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

from tts_webui import core_config_bridge as bridge


class Stub:
    def __init__(self, doc):
        self.doc = doc
        self.requests = []
        stub = self

        class H(BaseHTTPRequestHandler):
            def _auth(self):
                stub.requests.append((self.command, self.path, self.headers.get("X-TTS-Core-Token")))
                if self.headers.get("X-TTS-Core-Token") != "tok":
                    self.send_response(403)
                    self.end_headers()
                    self.wfile.write(b'{"error":"forbidden"}')
                    return False
                return True

            def do_GET(self):
                if self._auth():
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(json.dumps(stub.doc).encode())

            def do_PUT(self):
                if self._auth():
                    n = int(self.headers.get("Content-Length", 0))
                    stub.doc = json.loads(self.rfile.read(n))
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b'{"ok":true}')

            def log_message(self, *a):
                pass

        self.server = HTTPServer(("127.0.0.1", 0), H)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class BridgeTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub({"gradio_interface_options": {"server_port": 7770}})
        self.env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        self.env.start()
        # Fresh copies of the patched modules, restored afterwards.
        from tts_webui.config import _save_config as save_mod, load_config as load_mod

        self.save_mod, self.load_mod = save_mod, load_mod
        self.orig = (load_mod.load_config, save_mod._save_config, save_mod._save_json)
        bridge._installed = False

    def tearDown(self):
        self.load_mod.load_config, self.save_mod._save_config, self.save_mod._save_json = self.orig
        bridge._installed = False
        self.env.stop()
        self.stub.close()

    def test_noop_without_url(self):
        with mock.patch.dict(os.environ):
            os.environ.pop("TTS_CORE_URL")
            self.assertFalse(bridge.install())
        self.assertIs(self.load_mod.load_config, self.orig[0])

    def test_install_patches_both(self):
        self.assertTrue(bridge.install())
        self.assertIs(self.load_mod.load_config, bridge.load_config)
        self.assertIs(self.save_mod._save_config, bridge.save_config)
        self.assertEqual(
            self.load_mod.load_config()["gradio_interface_options"]["server_port"], 7770
        )

    def test_save_goes_to_go(self):
        bridge.install()
        self.save_mod._save_config({"a": 1})
        self.assertEqual(self.stub.doc, {"a": 1})
        self.save_mod._save_json("config.json", {"b": 2})
        self.assertEqual(self.stub.doc, {"b": 2})

    def test_save_json_other_file_untouched(self):
        import tempfile

        bridge.install()
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "other.json")
            self.save_mod._save_json(p, {"x": 1})
            with open(p) as f:
                self.assertEqual(json.load(f), {"x": 1})
        self.assertEqual(self.stub.doc["gradio_interface_options"]["server_port"], 7770)

    def test_late_importers_get_patched_functions(self):
        bridge.install()
        # What config_utils and server.py do: bind the name at import.
        from tts_webui.config._save_config import _save_config

        _save_config({"c": 3})
        self.assertEqual(self.stub.doc, {"c": 3})

    def test_bad_token_raises_no_file_fallback(self):
        bridge.install()
        os.environ["TTS_CORE_TOKEN"] = "wrong"
        with self.assertRaises(bridge.CoreConfigError):
            self.load_mod.load_config()
        with self.assertRaises(bridge.CoreConfigError):
            self.save_mod._save_config({})

    def test_unreachable_raises(self):
        bridge.install()
        self.stub.close()
        with self.assertRaises(bridge.CoreConfigError):
            self.load_mod.load_config()
        self.stub = Stub({})  # for tearDown


if __name__ == "__main__":
    unittest.main()
