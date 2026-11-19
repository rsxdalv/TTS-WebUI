"""unittest for the router bridge against a stub control API.

Run from the repo root:  python -m unittest tts_webui.core_router_bridge.test_core_router_bridge
"""

import json
import os
import sys
import threading
import types
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

from tts_webui import core_router_bridge as bridge


class Stub:
    """Records POSTs; answers with `status` (a list consumed per request)."""

    def __init__(self, statuses=None):
        self.requests = []
        self.statuses = list(statuses or [])
        stub = self

        class H(BaseHTTPRequestHandler):
            def do_POST(self):
                n = int(self.headers.get("Content-Length", 0))
                stub.requests.append(
                    (self.path, {k.lower(): v for k, v in self.headers.items()}, json.loads(self.rfile.read(n)))
                )
                code = stub.statuses.pop(0) if stub.statuses else 200
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b"{}")

            def log_message(self, *a):
                pass

        self.server = HTTPServer(("127.0.0.1", 0), H)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class RegisterRouteTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        patcher = mock.patch.object(bridge, "_RETRY_DELAY", 0)
        patcher.start()
        self.addCleanup(patcher.stop)

    def env(self, **kw):
        return {"TTS_CORE_URL": self.stub.url + "/", "TTS_CORE_TOKEN": "sekret", **kw}

    def test_posts_prefix_port_and_token(self):
        self.assertTrue(bridge.register_route("mms", 27771, self.env()))
        path, headers, body = self.stub.requests[0]
        self.assertEqual(path, "/api/routes")
        self.assertEqual(body, {"prefix": "/mms", "port": 27771})
        self.assertEqual(headers["x-tts-core-token"], "sekret")
        self.assertEqual(headers["content-type"], "application/json")

    def test_retries_server_errors_then_succeeds(self):
        self.stub.statuses = [500, 500, 200]
        self.assertTrue(bridge.register_route("mms", 1, self.env()))
        self.assertEqual(len(self.stub.requests), 3)

    def test_forbidden_is_not_retried_and_does_not_raise(self):
        self.stub.statuses = [403]
        with mock.patch("builtins.print") as p:
            self.assertFalse(bridge.register_route("mms", 1, self.env()))
        self.assertEqual(len(self.stub.requests), 1)
        self.assertIn("could not register route /mms", p.call_args[0][0])
        self.assertIn("403", p.call_args[0][0])

    def test_unreachable_core_does_not_raise(self):
        self.stub.close()
        with mock.patch("builtins.print") as p:
            self.assertFalse(bridge.register_route("mms", 1, self.env()))
        self.assertIn("could not register route", p.call_args[0][0])

    def test_no_url_does_not_raise(self):
        with mock.patch("builtins.print"):
            self.assertFalse(bridge.register_route("mms", 1, {}))


class InstallTest(unittest.TestCase):
    def setUp(self):
        self.opts = {"server_port": 7770, "server_name": "127.0.0.1", "inbrowser": True}
        self.main = types.ModuleType("tts_webui.gradio_proxy_tree.main")
        self.main.setup_gradio_proxy_tree = lambda o: "original"
        self.main.add_extension_route = lambda n, p: "original"
        cfg = types.ModuleType("tts_webui.config.config")
        cfg.config = {"gradio_interface_options": self.opts}
        fakes = {
            "tts_webui.gradio_proxy_tree.main": self.main,
            "tts_webui.config.config": cfg,
        }
        for name, mod in fakes.items():
            p = mock.patch.dict(sys.modules, {name: mod})
            p.start()
            self.addCleanup(p.stop)
        self.addCleanup(setattr, bridge, "_installed", False)
        bridge._installed = False
        p = mock.patch.object(sys, "argv", ["server_core.py", "--no-react", "--docker"])
        p.start()
        self.addCleanup(p.stop)

    def test_noop_without_core_url(self):
        with mock.patch.dict(os.environ, {}, clear=True):
            self.assertFalse(bridge.install())
            self.assertNotIn("GRADIO_TREE_URL", os.environ)
        self.assertEqual(self.opts["server_port"], 7770)
        self.assertEqual(self.main.add_extension_route("a", 1), "original")
        self.assertIn("--docker", sys.argv)

    def test_install_patches_everything(self):
        env = {
            "TTS_CORE_URL": "http://127.0.0.1:1",
            "TTS_GRADIO_PORT": "27700",
            "TTS_GRADIO_HOST": "0.0.0.0",
        }
        with mock.patch.dict(os.environ, env, clear=True), mock.patch("builtins.print"):
            self.assertTrue(bridge.install())
            self.assertEqual(os.environ["GRADIO_TREE_URL"], "")
        self.assertEqual(self.opts["server_port"], 27700)
        self.assertEqual(self.opts["server_name"], "0.0.0.0")
        self.assertIs(self.opts["inbrowser"], False)
        self.assertEqual(sys.argv, ["server_core.py", "--no-react", "--no-tree-proxy"])
        self.assertIsNone(self.main.setup_gradio_proxy_tree({"server_port": 1}))
        self.assertIsNot(self.main.add_extension_route, bridge.register_route)

    def test_keeps_existing_host_and_port_when_unset(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_URL": "http://x"}, clear=True), mock.patch("builtins.print"):
            bridge.install()
        self.assertEqual((self.opts["server_port"], self.opts["server_name"]), (7770, "127.0.0.1"))
        self.assertIs(self.opts["inbrowser"], False)

    def test_install_is_idempotent_and_no_tree_flag_not_duplicated(self):
        sys.argv[:] = ["x", "--no-tree-proxy"]
        with mock.patch.dict(os.environ, {"TTS_CORE_URL": "http://x"}, clear=True), mock.patch("builtins.print"):
            bridge.install()
            bridge.install()
        self.assertEqual(sys.argv.count("--no-tree-proxy"), 1)

    def test_patched_add_extension_route_posts_to_go(self):
        stub = Stub()
        self.addCleanup(stub.close)
        env = {"TTS_CORE_URL": stub.url, "TTS_CORE_TOKEN": "t"}
        with mock.patch.dict(os.environ, env, clear=True), mock.patch("builtins.print"):
            bridge.install()
            self.assertIsNone(self.main.add_extension_route("bark", 27772))
        self.assertEqual(stub.requests[0][2], {"prefix": "/bark", "port": 27772})


if __name__ == "__main__":
    unittest.main()
