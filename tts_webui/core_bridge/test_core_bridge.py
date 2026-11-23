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
    ("GET", "rows"): [
        ["2023-06-02T10:54:24", "Bark history: none", "3 days ago", "outputs/x/x.wav"],
        ["1970-01-01T00:00:00", "No_date_here", "Unknown", "outputs/y/y.wav"],
    ],
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
    # logs
    ("GET", "logs-files"): ["pip-install-a.log", "notes.log"],    ("GET", "logs-pip-logs"): [{"name": "a", "file": "pip-install-a.log"}],
    ("GET", "logs-stats"): "**Total Log Files:** 2",
    ("GET", "logs-summary"): "# Summary",
    ("GET", "logs-info"): "**File:** pip-install-a.log",
    ("GET", "logs-analyze"): "# Analysis",
    ("GET", "logs-read"): "line\n",
    ("POST", "logs-delete"): {"status": "Successfully deleted notes.log", "files": ["pip-install-a.log"]},
    ("POST", "logs-delete-all"): {"status": "Deleted 1 log file(s)", "files": []},
    # config
    ("POST", "extension-enable"): {"changed": True, "disabled": False, "info": "Enabled extension. Restart."},
    ("POST", "extension-disable"): {"changed": True, "disabled": True, "info": "Disabled extension. Restart."},
    ("POST", "extension-toggle"): {"changed": True, "disabled": True, "info": "Disabled extension. Restart."},
    ("POST", "extension-autostart"): {"changed": True, "enabled": True},
    ("POST", "set-value"): {"changed": True},
    ("POST", "replace-section"): {"changed": True},
}

def X(package_name, name, enabled=True, autostart=False, proxy=""):
    return {
        "package_name": package_name,
        "name": name,
        "requirements": "git+https://example.com/" + package_name,
        "proxy": proxy,
        "enabled": enabled,
        "autostart": autostart,
    }


# extcatalog resolve: (kind, extension_class) -> what Go lists to load
RESOLVED = {
    ("interface", "text-to-speech"): {
        "extensions": [
            X("x.builtin.alpha", "Alpha"),
            X("x.builtin.beta", "Beta", enabled=False),
            X("x.builtin.gamma", "Gamma"),
        ]
    },
    ("decorator", "outer"): {
        "extensions": [X("deco.one", "One"), X("deco.off", "Off", enabled=False)],
        "disabled_names": ["deco.off", "decorator_skipped"],
    },
}

# POST ops that answer with NDJSON lines
STREAMS = {
    "install": [{"line": "Collecting x"}, {"line": "Installed x"}, {"done": True, "ok": True}],
    "install-entry": [
        {"line": "Collecting x"},
        {"line": "Installed x"},
        {"done": True, "ok": True},
    ],
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
                if u.path.startswith("/api/logs/"):
                    op = "logs-" + op  # "delete" etc. are also history ops
                if u.path == "/api/pip/list":
                    op = "pip-list"  # "list" is also an extcatalog op
                token = self.headers.get("X-TTS-Core-Token")
                query = {k: v[0] for k, v in parse_qs(u.query).items()}
                stub.requests.append((self.command, op, query, body, token))
                if token != "tok":
                    code, out = 403, {"error": "forbidden"}
                elif op == "delete" and body["directory"] == "gone":
                    code, out = 404, {"error": "not found: gone"}
                elif op == "resolve":
                    code, out = 200, RESOLVED.get(
                        (query["kind"], query["extension_class"]),
                        {"extensions": []},
                    )
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


class HistoryTabTest(unittest.TestCase):
    """The History tab (Outputs > History and its Favorites and Collections
    siblings) is thin Gradio over the Go history API."""

    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def test_rows_come_from_go(self):
        rows = bridge.history.rows("outputs")
        self.assertEqual(rows, ANSWERS[("GET", "rows")])
        self.assertEqual(self.stub.requests[-1][1], "rows")
        self.assertEqual(self.stub.requests[-1][2], {"directory": "outputs"})

    def test_rows_fit_the_dataframe(self):
        # The tab hands the rows to gr.Dataframe as they are.
        rows = bridge.history.rows("outputs")
        df = gr.Dataframe(
            value=rows,
            type="array",
            col_count=4,
            datatype=["date", "str", "str", "str"],
        )
        self.assertEqual(df.postprocess(rows).data, rows)

    def test_delete_generation_goes_through_go(self):
        from tts_webui.history_tab.delete_generation import delete_generation

        self.stub.requests.clear()
        self.assertEqual(delete_generation("outputs/x"), {"ok": True})
        self.assertEqual(self.stub.requests[-1][1], "delete")
        self.assertEqual(self.stub.requests[-1][3], {"directory": "outputs/x"})

    def test_tab_builds_and_keeps_the_api_names(self):
        import importlib

        m = importlib.import_module("extensions.builtin.extension_history_tab.main")
        with gr.Blocks() as demo:
            m.extension__tts_generation_webui()
        names = {f.api_name for f in demo.fns.values() if f.api_name}
        for want in (
            "outputs_refresh_history",
            "favorites_refresh_history",
            "collections_refresh_history",
            "save_to_voices",
            "delete_generation",
            "open_folder",
        ):
            self.assertIn(want, names)

    def test_tab_has_no_filesystem_logic_left(self):
        import importlib

        m = importlib.import_module("extensions.builtin.extension_history_tab.main")
        with open(m.__file__, encoding="utf-8") as f:
            src = f.read()
        for gone in (
            "shutil",
            "os.makedirs",
            "open(",
            "json.load",
            "get_wav_files",
            "generate_pretty_name",
            "generate_relative_date",
        ):
            self.assertNotIn(gone, src)


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

    def test_install_entry_takes_the_entry(self):
        entry = {"package_name": "n.one", "requirements": "git+https://x/y@main"}
        out = list(bridge.extcatalog.install_entry(entry))
        self.assertEqual(out, ["Collecting x", "Collecting x<br />Installed x"])
        self.assertEqual(self.stub.requests[0][1], "install-entry")
        self.assertEqual(self.stub.requests[0][3], {"entry": entry})
        with self.assertRaises(TypeError):
            list(bridge.extcatalog.install_entry())
        with self.assertRaises(TypeError):
            list(bridge.extcatalog.install_entry({"package_name": "a"}, "b"))

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


class ExtensionCatalogTabTest(unittest.TestCase):
    """The Extension Catalog tab (extensions/builtin/extension_extension_catalog)
    is thin Gradio over the Go extcatalog API: it parses, previews, adds and
    installs through the bridge, and holds no JSON or file logic of its own."""

    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)
        import importlib

        self.m = importlib.import_module(
            "extensions.builtin.extension_extension_catalog.main"
        )

    def test_parse_goes_through_go(self):
        entries, preview, info = self.m._on_parse('{"package_name": "n.one"}')
        self.assertEqual(entries, [{"package_name": "n.one"}])
        self.assertEqual(info, "Parsed entries: 1")
        self.assertIsInstance(preview, gr.Markdown)
        self.assertEqual(self.stub.requests[-1][1], "parse")
        self.assertEqual(self.stub.requests[-1][3], {"text": '{"package_name": "n.one"}'})

    def test_add_goes_through_go(self):
        status, shown = self.m._add_to_external([{"package_name": "n.one"}])
        self.assertTrue(status.startswith("Saved"))
        self.assertEqual(shown, "{}")
        self.assertEqual(self.stub.requests[-1][1], "add")
        self.assertEqual(self.stub.requests[-1][3], {"entries": [{"package_name": "n.one"}]})

    def test_install_goes_through_go_with_the_entry(self):
        entry = {"package_name": "n.one", "requirements": "git+https://x/y@main"}
        out = list(self.m._install_selected([entry]))
        self.assertEqual(out, ["Collecting x", "Collecting x<br />Installed x"])
        self.assertEqual(self.stub.requests[-1][1], "install-entry")
        self.assertEqual(self.stub.requests[-1][3], {"entry": entry})

    def test_install_of_nothing_says_so(self):
        self.assertEqual(
            list(self.m._install_selected([])), ["<i>Nothing selected to install.</i>"]
        )
        self.assertEqual(len(self.stub.requests), 0)

    def test_tab_builds(self):
        with gr.Blocks() as demo:
            info = self.m.extension__tts_generation_webui()
        self.assertEqual(info["package_name"], "extensions.builtin.extension_extension_catalog")
        kinds = [type(b).__name__ for b in demo.blocks.values()]
        self.assertIn("HTML", kinds)  # the iframe and the console
        self.assertIn("Markdown", kinds)  # the preview
        self.assertIn("Code", kinds)  # the current extensions.external.json

    def test_tab_has_no_json_or_file_logic_left(self):
        with open(self.m.__file__, encoding="utf-8") as f:
            src = f.read()
        for gone in (
            "json.load",
            "json.dump",
            "json.loads",
            "json.JSONDecodeError",
            "REQUIRED_KEYS",
            "_validate_entry",
            "_parse_json_input",
            "_render_preview",
            "_load_external_extensions",
            "_save_external_extensions",
            "pip_install_wrapper",
            "venv_setup_wrapper",
            "open(",
            "EXTERNAL_EXTENSIONS_FILE",
        ):
            self.assertNotIn(gone, src)

    def test_core_off_raises(self):
        with mock.patch.dict(os.environ, clear=False):
            os.environ.pop("TTS_CORE_URL")
            with self.assertRaises(CoreBridgeError) as cm:
                self.m._on_parse("{}")
            self.assertIn("TTS_CORE_URL", str(cm.exception))
            with self.assertRaises(CoreBridgeError):
                self.m._add_to_external([])
            with self.assertRaises(CoreBridgeError):
                list(self.m._install_selected([{"package_name": "n.one"}]))

    def test_go_error_is_raised(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError):
                self.m._on_parse("{}")


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
                lambda: bridge.history.items("outputs"),
                lambda: bridge.history.rows("outputs"),
                lambda: bridge.history.select(path="outputs/x/x.png"),
                lambda: bridge.history.delete(directory="outputs/x"),
                lambda: bridge.history.copy("outputs/x", "favorites"),
                lambda: bridge.history.save_to_voices(npz="outputs/x/x.npz"),
                lambda: bridge.extcatalog.parse("{}"),
                lambda: bridge.extcatalog.add([]),
                lambda: list(bridge.extcatalog.install("a.one")),
                lambda: list(bridge.extcatalog.install_entry({"package_name": "a.one"})),
                lambda: bridge.extcatalog.list("interface"),
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


class ExtensionLoadingTest(unittest.TestCase):
    """Go lists what to load (bridge.extcatalog.resolve); Python imports and
    registers exactly that."""

    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def _tab_labels(self, import_module):
        from tts_webui.extensions_loader import interface_extensions as ie

        with mock.patch.object(ie, "check_if_package_installed", return_value=True), \
                mock.patch.object(ie.importlib, "import_module", import_module), \
                gr.Blocks() as demo, gr.Tabs():
            ie.handle_extension_class("text-to-speech")
        return [b.label for b in demo.blocks.values() if isinstance(b, gr.Tab)]

    def test_tabs_follow_go_order_and_state(self):
        ran = []
        fake = mock.Mock(extension__tts_generation_webui=lambda: ran.append(1))
        import_module = mock.Mock(return_value=fake)
        labels = self._tab_labels(import_module)
        self.assertEqual(labels, ["Alpha", "[Disabled] Beta", "Gamma"])
        self.assertEqual(
            [c.args[0] for c in import_module.call_args_list],
            ["x.builtin.alpha.main", "x.builtin.gamma.main"],  # a disabled one is never imported
        )
        self.assertEqual(len(ran), 2)
        self.assertEqual(
            self.stub.requests[0][:3],
            ("GET", "resolve", {"kind": "interface", "extension_class": "text-to-speech"}),
        )

    def test_a_failing_import_leaves_an_error_tab(self):
        def boom(name):
            raise ImportError("no module " + name)

        labels = self._tab_labels(boom)
        self.assertEqual(labels, ["Alpha", "Alpha (!)", "[Disabled] Beta", "Gamma", "Gamma (!)"])

    def test_the_loader_keeps_no_copy_of_the_config_lists(self):
        from tts_webui.extensions_loader import interface_extensions as ie

        for gone in ("disabled_extensions", "autostart_extensions", "extension_list_json"):
            self.assertFalse(hasattr(ie, gone), gone)

    def _decorators(self):
        import importlib

        return importlib.import_module("tts_webui.extensions_loader.decorator_extensions")

    def test_decorators_load_what_go_lists(self):
        de = self._decorators()
        module = mock.Mock(spec=[])
        module.decorator_plain = lambda fn: fn
        module.decorator_gen_generator = lambda fn: fn
        module.decorator_skipped = lambda fn: fn  # named in disabled_decorators
        module.helper = lambda: None
        with mock.patch.object(de.importlib, "import_module", return_value=module) as imp:
            wrappers, gen_wrappers = de._load_decorators("outer")
        self.assertEqual([c.args[0] for c in imp.call_args_list], ["deco.one.main"])
        self.assertEqual(wrappers, [module.decorator_plain])
        self.assertEqual(gen_wrappers, [module.decorator_gen_generator])

    def test_decorator_output_and_failed_import(self):
        import contextlib
        import io

        de = self._decorators()
        out = io.StringIO()
        with contextlib.redirect_stdout(out), mock.patch.object(
            de.importlib, "import_module", side_effect=ImportError("nope")
        ):
            self.assertEqual(de._load_decorators("outer"), ([], []))
        text = out.getvalue()
        self.assertIn("Skipping disabled decorator extension Off", text)
        self.assertIn("Failed to load: nope", text)
        self.assertNotIn("Failed to load: cannot unpack", text)  # the old loader's message for a disabled one

    def test_extension_processes_get_the_list_from_go_through_the_environment(self):
        de = self._decorators()
        handed = {"outer": {"extensions": [X("deco.env", "Env")]}, "inner": {"extensions": []}}
        env = {k: v for k, v in os.environ.items() if k != "TTS_CORE_URL"}  # no control API there
        env[de.DECORATORS_ENV] = json.dumps(handed)
        module = mock.Mock(spec=[])
        module.decorator_env = lambda fn: fn
        asked = len(self.stub.requests)
        with mock.patch.dict(os.environ, env, clear=True), mock.patch.object(
            de.importlib, "import_module", return_value=module
        ) as imp:
            wrappers, _ = de._load_decorators("outer")
        self.assertEqual(wrappers, [module.decorator_env])
        self.assertEqual(imp.call_args.args[0], "deco.env.main")
        self.assertEqual(len(self.stub.requests), asked)  # no call to Go

    def test_core_off_raises(self):
        env = {k: v for k, v in os.environ.items() if k != "TTS_CORE_URL"}
        with mock.patch.dict(os.environ, env, clear=True):
            with self.assertRaises(CoreBridgeError):
                bridge.extcatalog.resolve("interface", "tools")
            from tts_webui.extensions_loader import interface_extensions as ie

            with self.assertRaises(CoreBridgeError):
                ie.handle_extension_class("tools")


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


class LogViewerTest(unittest.TestCase):
    def setUp(self):
        self.stub = Stub()
        self.addCleanup(self.stub.close)
        env = mock.patch.dict(
            os.environ, {"TTS_CORE_URL": self.stub.url, "TTS_CORE_TOKEN": "tok"}
        )
        env.start()
        self.addCleanup(env.stop)

    def _load(self):
        import importlib.util
        import sys

        spec = importlib.util.spec_from_file_location(
            "log_viewer_main",
            os.path.join("extensions", "builtin", "extension_log_viewer", "main.py"),
        )
        mod = importlib.util.module_from_spec(spec)
        sys.modules["log_viewer_main"] = mod
        spec.loader.exec_module(mod)
        return mod

    def test_ops_send_named_fields(self):
        b = bridge.logs
        self.assertEqual(b.files(), ["pip-install-a.log", "notes.log"])
        self.assertEqual(b.pip_logs("install")[0]["file"], "pip-install-a.log")
        b.read("a.log", "err", 500)
        b.delete("a.log")
        b.delete_all()
        got = [(r[0], r[1], r[2], r[3]) for r in self.stub.requests]
        self.assertEqual(got[1], ("GET", "logs-pip-logs", {"kind": "install"}, None))
        self.assertEqual(
            got[2], ("GET", "logs-read", {"file": "a.log", "search": "err", "max_lines": "500"}, None)
        )
        self.assertEqual(got[3], ("POST", "logs-delete", {}, {"file": "a.log"}))
        self.assertEqual(got[4], ("POST", "logs-delete-all", {}, {}))

    def test_tab_builds_and_handlers(self):
        mod = self._load()
        with gr.Blocks():
            info = mod.extension__tts_generation_webui()
        self.assertEqual(info["package_name"], "extensions.builtin.extension_log_viewer")
        self.assertEqual(mod._choices("install"), [("a", "pip-install-a.log")])
        self.assertEqual(mod._read("", "x", 10), "No file selected")
        self.assertEqual(mod._read("a.log", None, None), "line\n")
        self.assertEqual(self.stub.requests[-1][2]["max_lines"], "1000")
        self.assertEqual(mod._info_and_analysis(None), ("No file selected", "Select a log file to see analysis"))
        self.assertEqual(mod._info_and_content("a.log", "", 100)[0], "**File:** pip-install-a.log")

    def test_core_off_and_go_error(self):
        with mock.patch.dict(os.environ, {"TTS_CORE_TOKEN": "bad"}):
            with self.assertRaises(CoreBridgeError):
                bridge.logs.files()
        with mock.patch.dict(os.environ, clear=False):
            os.environ.pop("TTS_CORE_URL")
            with self.assertRaises(CoreBridgeError) as cm:
                bridge.logs.stats()
            self.assertIn("TTS_CORE_URL", str(cm.exception))
            with self.assertRaises(CoreBridgeError):
                self._load()._read("a.log", "", 10)


if __name__ == "__main__":
    unittest.main()
