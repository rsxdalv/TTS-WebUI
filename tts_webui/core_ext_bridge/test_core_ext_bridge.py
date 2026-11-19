"""Tests for tts_webui.core_ext_bridge (stdlib unittest, stub HTTP server).

Run from the repo root:
    python -m unittest tts_webui.core_ext_bridge.test_core_ext_bridge -v
"""

import json
import os
import subprocess
import sys
import tempfile
import textwrap
import threading
import types
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from tts_webui import core_ext_bridge as bridge

TOKEN = "s3cret"
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class StubCore:
    """Minimal fake of Go's /api/extensions API."""

    def __init__(self):
        self.requests = []  # (method, path, headers, body)
        self.start_response = (200, {"port": 0, "pid": 4242})
        self.stop_response = (200, {"stopped": True})
        self.list_response = (200, [])
        outer = self

        class H(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def _do(self):
                n = int(self.headers.get("Content-Length") or 0)
                raw = self.rfile.read(n) if n else b""
                body = json.loads(raw) if raw else None
                outer.requests.append((self.command, self.path, {k.title(): v for k, v in self.headers.items()}, body))
                if self.headers.get("X-TTS-Core-Token") != TOKEN:
                    code, data = 403, {"error": "bad token"}
                elif self.path.endswith("/start"):
                    code, data = outer.start_response
                    if code == 200 and data.get("port") == 0:
                        data = dict(data, port=body.get("port", 1))
                elif self.path.endswith("/stop"):
                    code, data = outer.stop_response
                else:
                    code, data = outer.list_response
                out = json.dumps(data).encode()
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(out)))
                self.end_headers()
                self.wfile.write(out)

            do_GET = do_POST = _do

        self.server = HTTPServer(("127.0.0.1", 0), H)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


def fake_module():
    """A stand-in with the globals the real setup_proxy_extension has."""
    mod = types.ModuleType("fake_setup_proxy_extension")
    mod.subprocess = subprocess
    mod.calls = []
    mod.wait_for_server = lambda port, timeout=30: mod.calls.append(("wait", port)) or True
    mod.add_extension_route = lambda name, port: mod.calls.append(("route", name, port))
    return mod


class Base(unittest.TestCase):
    def setUp(self):
        self.core = StubCore()
        self.addCleanup(self.core.close)
        self.addCleanup(bridge.uninstall)
        bridge.uninstall()
        self.env = {"TTS_CORE_URL": self.core.url, "TTS_CORE_TOKEN": TOKEN}

    def installed_module(self):
        self.assertTrue(bridge.install(self.env))
        mod = fake_module()
        bridge._patch_module(mod)
        return mod

    @staticmethod
    def ext_env(**extra):
        return {
            "PATH": "x",
            "GRADIO_SERVER_PORT": "27771",
            "GRADIO_ROOT_PATH": "/pkg",
            "TTS_WEBUI_EXTENSION_PACKAGE": "pkg",
            "OPENAI_PROXY_HOST": "http://localhost:7778",
            **extra,
        }


class InstallTests(Base):
    def test_noop_without_core_url(self):
        before = list(sys.meta_path)
        self.assertFalse(bridge.install({}))
        self.assertFalse(bridge.install({"TTS_CORE_TOKEN": "x"}))
        self.assertEqual(before, sys.meta_path)
        self.assertEqual({}, bridge._state)

    def test_install_registers_import_hook_and_is_idempotent(self):
        self.assertTrue(bridge.install(self.env))
        finder = bridge._state["finder"]
        self.assertIn(finder, sys.meta_path)
        self.assertTrue(bridge.install(self.env))
        self.assertEqual(1, sys.meta_path.count(finder))

    def test_patches_already_imported_module(self):
        name = "fake_already_imported_mod"
        mod = fake_module()
        sys.modules[name] = mod
        self.addCleanup(sys.modules.pop, name, None)
        self.assertIsNone(bridge._install_patch(name, bridge._patch_module))
        self.assertIsInstance(mod.subprocess, bridge._SubprocessShim)

    def test_import_hook_patches_after_first_import(self):
        with tempfile.TemporaryDirectory() as d:
            with open(os.path.join(d, "hooked_mod_for_test.py"), "w") as f:
                f.write("import subprocess\nORIGINAL = subprocess\n")
            sys.path.insert(0, d)
            self.addCleanup(sys.path.remove, d)
            self.addCleanup(sys.modules.pop, "hooked_mod_for_test", None)
            finder = bridge._install_patch(
                "hooked_mod_for_test", lambda m: setattr(m, "patched", True)
            )
            self.assertIsNotNone(finder)
            import hooked_mod_for_test

            self.assertTrue(hooked_mod_for_test.patched)
            self.assertNotIn(finder, sys.meta_path)
            self.assertIs(subprocess, hooked_mod_for_test.ORIGINAL)


class PopenTests(Base):
    def test_start_request_and_process_object(self):
        mod = self.installed_module()
        env = self.ext_env(HF_HOME="/models", FOO="bar")
        bridge._state["env0"] = {"PATH": "x", "FOO": "bar", "GRADIO_SERVER_PORT": "7"}
        proc = mod.subprocess.Popen(["python", "-m", "harness"], env=env)

        method, path, headers, body = self.core.requests[-1]
        self.assertEqual(("POST", "/api/extensions/pkg/start"), (method, path))
        self.assertEqual(TOKEN, headers["X-Tts-Core-Token"])
        self.assertEqual("application/json", headers["Content-Type"])
        self.assertEqual(27771, body["port"])
        self.assertEqual("http://localhost:7778", body["openai_proxy_host"])
        # only variables changed since install(), without the handled ones
        self.assertEqual({"HF_HOME": "/models"}, body["env"])
        self.assertEqual(4242, proc.pid)
        self.assertIsNone(proc.returncode)

    def test_token_and_url_never_forwarded(self):
        mod = self.installed_module()
        bridge._state["env0"] = {}
        mod.subprocess.Popen(["p"], env=self.ext_env(TTS_CORE_TOKEN="t", TTS_CORE_URL="u"))
        self.assertNotIn("TTS_CORE_TOKEN", self.core.requests[-1][3]["env"])
        self.assertNotIn("TTS_CORE_URL", self.core.requests[-1][3]["env"])

    def test_terminate_calls_stop(self):
        mod = self.installed_module()
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        proc.terminate()
        self.assertEqual(("POST", "/api/extensions/pkg/stop"), self.core.requests[-1][:2])
        self.assertEqual(0, proc.returncode)

    def test_terminate_ignores_404(self):
        mod = self.installed_module()
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        self.core.stop_response = (404, {"error": "extension not found"})
        proc.terminate()

    def test_terminate_error_is_clear(self):
        mod = self.installed_module()
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        self.core.stop_response = (500, {"error": "did not exit"})
        with self.assertRaises(Exception) as cm:
            proc.terminate()
        self.assertIn("did not exit", str(cm.exception))

    def test_poll_and_wait(self):
        mod = self.installed_module()
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        self.core.list_response = (200, [{"package": "pkg", "pid": 4242, "state": "ready"}])
        self.assertIsNone(proc.poll())
        with self.assertRaises(subprocess.TimeoutExpired):
            proc.wait(timeout=0.3)
        self.core.list_response = (
            200,
            [{"package": "pkg", "pid": 4242, "state": "exited", "exit_code": 3}],
        )
        self.assertEqual(3, proc.poll())
        self.assertEqual(3, proc.wait())

    def test_poll_unknown_when_core_unreachable(self):
        mod = self.installed_module()
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        self.core.close()
        self.assertIsNone(proc.poll())

    def test_start_http_error_surfaces_message(self):
        mod = self.installed_module()
        self.core.start_response = (500, {"error": "spawning pkg: no python"})
        with self.assertRaises(Exception) as cm:
            mod.subprocess.Popen(["p"], env=self.ext_env())
        msg = str(cm.exception)
        self.assertIn("pkg", msg)
        self.assertIn("no python", msg)

    def test_unreachable_core_is_clear_not_a_crash(self):
        self.env["TTS_CORE_URL"] = "http://127.0.0.1:1"
        mod = self.installed_module()
        with self.assertRaises(Exception) as cm:
            mod.subprocess.Popen(["p"], env=self.ext_env())
        self.assertIn("cannot reach the TTS core", str(cm.exception))

    def test_bad_token_reports_403(self):
        self.env["TTS_CORE_TOKEN"] = "wrong"
        mod = self.installed_module()
        with self.assertRaises(Exception) as cm:
            mod.subprocess.Popen(["p"], env=self.ext_env())
        self.assertIn("403", str(cm.exception))

    def test_non_extension_popen_falls_through(self):
        mod = self.installed_module()
        p = mod.subprocess.Popen([sys.executable, "-c", "pass"], env=dict(os.environ))
        self.assertEqual(0, p.wait())
        self.assertEqual([], self.core.requests)

    def test_shim_delegates_other_attributes(self):
        mod = self.installed_module()
        self.assertIs(subprocess.PIPE, mod.subprocess.PIPE)
        self.assertIs(subprocess.run, mod.subprocess.run)

    def test_port_mismatch_translates_wait_and_route(self):
        mod = self.installed_module()
        self.core.start_response = (200, {"port": 30000, "pid": 9, "already_running": True})
        proc = mod.subprocess.Popen(["p"], env=self.ext_env())
        self.assertEqual(30000, proc.port)
        mod.wait_for_server(27771, timeout=5)
        mod.add_extension_route("pkg", 27771)
        mod.add_extension_route("other", 27999)
        self.assertEqual(
            [("wait", 30000), ("route", "pkg", 30000), ("route", "other", 27999)],
            mod.calls,
        )


REAL_MODULE_SCRIPT = """
import json, os, sys
from tts_webui import core_ext_bridge as b
assert b.install() is True
import gradio  # the real module needs it
import tts_webui.extensions_loader.setup_proxy_extension as m
assert isinstance(m.subprocess, b._SubprocessShim), m.subprocess
env = dict(os.environ, GRADIO_SERVER_PORT="27771", GRADIO_ROOT_PATH="/pkg",
           TTS_WEBUI_EXTENSION_PACKAGE="pkg", OPENAI_PROXY_HOST="http://localhost:7778")
p = m.subprocess.Popen([sys.executable, "-m", "tts_webui.extensions_loader.proxy_harness"], env=env)
assert p.pid == 4242, p.pid
p.terminate()
print("OK")
"""


class RealModuleTest(Base):
    """Imports the real setup_proxy_extension in a child interpreter."""

    def test_real_setup_proxy_extension_is_patched(self):
        try:
            import gradio  # noqa: F401
        except ImportError:
            self.skipTest("gradio not installed")
        with tempfile.TemporaryDirectory() as cwd:
            env = dict(
                os.environ,
                PYTHONPATH=REPO,
                TTS_CORE_URL=self.core.url,
                TTS_CORE_TOKEN=TOKEN,
                GRADIO_ANALYTICS_ENABLED="False",
            )
            r = subprocess.run(
                [sys.executable, "-c", textwrap.dedent(REAL_MODULE_SCRIPT)],
                cwd=cwd, env=env, capture_output=True, text=True, timeout=180,
            )
        self.assertEqual(0, r.returncode, r.stdout + r.stderr)
        self.assertIn("OK", r.stdout)
        self.assertEqual(
            ["/api/extensions/pkg/start", "/api/extensions/pkg/stop"],
            [x[1] for x in self.core.requests],
        )


if __name__ == "__main__":
    unittest.main()
