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
    # extcatalog
    ("GET", "list"): [{"package_name": "a.one", "name": "A One"}],
    ("GET", "table"): M("Markdown", value="| Title | Description |"),
    ("GET", "picker"): M("Dropdown", label="Pick", choices=["a.one", "b.two"]),
    ("POST", "parse"): {
        "entries": [{"package_name": "n.one"}],
        "preview": M("Markdown", value="| Name |"),
        "info": "Parsed entries: 1",
    },
    ("POST", "add"): {"status": "Saved | Added: n.one | Skipped (exists): none", "json": "{}"},
    ("GET", "pip-list"): [{"name": "Alpha", "version": "1.0"}, {"name": "numpy", "version": "2.0"}],
    # config
    ("POST", "extension-enable"): {"changed": True, "disabled": False, "info": "Enabled extension. Restart."},
    ("POST", "extension-disable"): {"changed": True, "disabled": True, "info": "Disabled extension. Restart."},
    ("POST", "extension-toggle"): {"changed": True, "disabled": True, "info": "Disabled extension. Restart."},
    ("POST", "extension-autostart"): {"changed": True, "enabled": True},
    ("POST", "set-value"): {"changed": True},
    ("POST", "replace-section"): {"changed": True},
}

# POST ops that answer with NDJSON lines
STREAMS = {
    "install": [{"line": "Collecting x"}, {"line": "Installed x"}, {"done": True, "ok": True}],
    "uninstall": [{"line": "Removed x"}, {"done": True, "ok": True}],
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
                op = u.path.rsplit("/", 1)[-1]
                if u.path == "/api/pip/list":
                    op = "pip-list"  # "list" is also an extcatalog op
                token = self.headers.get("X-TTS-Core-Token")
                query = {k: v[0] for k, v in parse_qs(u.query).items()}
                stub.requests.append((self.command, op, query, body, token))
                if token != "tok":
                    code, out = 403, {"error": "forbidden"}
                elif op == "delete" and body["directory"] == "gone":
                    code, out = 404, {"error": "not found: gone"}
                elif op == "install" and body["package_name"] == "missing":
                    code, out = 404, {"error": 'not found: no installable extension "missing"'}
                elif self.command == "POST" and op in STREAMS:
                    self.send_response(200)
                    self.send_header("Content-Type", "application/x-ndjson")
                    self.end_headers()
                    for rec in STREAMS[op]:
                        self.wfile.write((json.dumps(rec) + chr(10)).encode())
                        self.wfile.flush()
                    return
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


class ExtCatalogTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_read_ops(self):
        e = bridge.extcatalog
        self.assertEqual(e.list("interface"), [{"package_name": "a.one", "name": "A One"}])
        self.assertEqual(self.stub.requests[0][2], {"kind": "interface"})
        with gr.Blocks():
            md = e.table("decorator")
            dd = e.picker("interface", "Pick")
        self.assertIsInstance(md, gr.Markdown)
        self.assertIsInstance(dd, gr.Dropdown)
        self.assertEqual([c[0] for c in dd.choices], ["a.one", "b.two"])
        self.assertEqual(self.stub.requests[2][2], {"kind": "interface", "label": "Pick"})

    def test_parse_and_add(self):
        res = bridge.extcatalog.parse("{}")
        self.assertEqual(res["info"], "Parsed entries: 1")
        self.assertIsInstance(res["preview"], gr.Markdown)
        self.assertEqual(self.stub.requests[0][3], {"text": "{}"})
        res = bridge.extcatalog.add([{"package_name": "n.one"}])
        self.assertTrue(res["status"].startswith("Saved"))
        self.assertEqual(self.stub.requests[1][3], {"entries": [{"package_name": "n.one"}]})

    def test_install_streams_the_console_html(self):
        import inspect

        install = bridge.extcatalog.install
        self.assertTrue(inspect.isgeneratorfunction(install))  # Gradio streams generators
        self.assertEqual(
            list(install("a.one")),
            ["Collecting x", "Collecting x<br />Installed x"],
        )
        self.assertEqual(list(bridge.extcatalog.uninstall(package_name="a.one")), ["Removed x"])
        self.assertEqual(self.stub.requests[0][3], {"package_name": "a.one"})

    def test_errors_before_the_stream_raise(self):
        with self.assertRaises(CoreBridgeError) as cm:
            list(bridge.extcatalog.install("missing"))
        self.assertIn("no installable extension", str(cm.exception))
        with self.assertRaises(TypeError):
            list(bridge.extcatalog.install())

    def test_list_tab_builds(self):
        from tts_webui.extensions_loader.list_tab import extension_list_ui

        with gr.Blocks() as demo:
            extension_list_ui("interface")
        kinds = [type(b).__name__ for b in demo.blocks.values()]
        self.assertEqual(kinds.count("Dropdown"), 2)
        self.assertIn("HTML", kinds)
        self.assertGreaterEqual(kinds.count("Markdown"), 3)  # intro, table, two headings
        self.assertEqual(sum(1 for f in demo.fns.values() if f.api_name and "extension" in f.api_name), 2)

    def test_custom_installer_handlers(self):
        import importlib

        m = importlib.import_module("extensions.builtin.extension_custom_extensions_installer.main")
        entries, preview, info = m._on_parse("{}")
        self.assertEqual((entries, info), ([{"package_name": "n.one"}], "Parsed entries: 1"))
        self.assertIsInstance(preview, gr.Markdown)
        self.assertEqual(m._add_to_external(entries)[1], "{}")
        out = list(m._install_selected([{"package_name": "a.one"}, {"package_name": "a.one"}]))
        self.assertEqual(out, ["Collecting x", "Collecting x<br />Installed x"] * 2)
        self.assertEqual(list(m._install_selected([])), ["<i>Nothing selected to install.</i>"])


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


class ConfigTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_ops_send_named_fields(self):
        c = bridge.config
        self.assertTrue(c.extension_disable("ext.a")["disabled"])
        self.assertEqual(c.extension_autostart("ext.a", True)["enabled"], True)
        c.set_value("ns", "key", {"a": [1, None]})
        c.replace_section("gradio_interface_options", {"server_port": 7771})
        self.assertEqual(
            [(r[1], r[3]) for r in self.stub.requests],
            [
                ("extension-disable", {"package_name": "ext.a"}),
                ("extension-autostart", {"package_name": "ext.a", "enabled": True}),
                ("set-value", {"namespace": "ns", "key": "key", "value": {"a": [1, None]}}),
                (
                    "replace-section",
                    {"namespace": "gradio_interface_options", "values": {"server_port": 7771}},
                ),
            ],
        )
        with self.assertRaises(TypeError):
            c.set_value("ns", "key")
        with self.assertRaises(AttributeError):
            c.save_everything

    def test_extension_handlers(self):
        from tts_webui.extensions_loader import interface_extensions as ie

        with mock.patch.object(ie.gr, "Info") as info:
            self.assertEqual(ie.toggle_extension_state("ext.a")(), "Enable")
            ie.enable_extension("ext.a")()
            ie.disable_extension("ext.a")()
            ie.toggle_autostart("ext.a")(False)
        self.assertEqual(info.call_count, 3)
        info.assert_any_call("Disabled extension. Restart.")
        self.assertEqual(
            [r[1] for r in self.stub.requests],
            ["extension-toggle", "extension-enable", "extension-disable", "extension-autostart"],
        )
        self.assertEqual(self.stub.requests[3][3], {"package_name": "ext.a", "enabled": False})

    def test_set_config_value_and_gradio_settings(self):
        from tts_webui.config.config import config
        from tts_webui.config.config_utils import get_config_value, set_config_value
        from tts_webui.config.save_config_gradio import save_config_gradio

        with mock.patch.dict(config, {}, clear=False):
            set_config_value("ns_test", "k", 5)
            self.assertEqual(get_config_value("ns_test", "k"), 5)
            msg = save_config_gradio(["server_port", "auth"], ["7771", ["u", "p"]])
            self.assertIn("7771", msg)
            config.pop("ns_test", None)
        self.assertEqual(self.stub.requests[0][3], {"namespace": "ns_test", "key": "k", "value": 5})
        self.assertEqual(
            self.stub.requests[1][3],
            {
                "namespace": "gradio_interface_options",
                "values": {"server_port": 7771, "auth": "u:p"},
            },
        )

    def test_core_off_raises(self):
        with mock.patch.dict(os.environ, clear=False):
            os.environ.pop("TTS_CORE_URL")
            with self.assertRaises(CoreBridgeError) as cm:
                bridge.config.extension_enable("ext.a")
            self.assertIn("TTS_CORE_URL", str(cm.exception))
            from tts_webui.config.config_utils import set_config_value

            with self.assertRaises(CoreBridgeError):
                set_config_value("ns", "k", 1)

    def test_go_error_is_raised(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError):
                bridge.config.extension_enable("ext.a")


class PipTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_installed_packages_tab(self):
        import importlib.util
        import sys

        spec = importlib.util.spec_from_file_location(
            "pip_list_main", os.path.join("extensions", "builtin", "extension_pip_list", "main.py")
        )
        mod = importlib.util.module_from_spec(spec)
        sys.modules["pip_list_main"] = mod
        spec.loader.exec_module(mod)
        got = mod.get_pip_list()
        self.assertEqual(got[0], {"name": "Alpha", "version": "1.0"})
        self.assertEqual(mod.refresh_pip_list(), "Alpha==1.0\n\nnumpy==2.0")
        self.assertEqual(self.stub.requests[0][2], {})

    def test_core_off_and_pip_failure(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError):
                bridge.pip.list()
        with mock.patch.dict(os.environ, clear=False):
            os.environ.pop("TTS_CORE_URL")
            with self.assertRaises(CoreBridgeError) as cm:
                bridge.pip.list()
            self.assertIn("TTS_CORE_URL", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
