"""unittest for the thin Go-core entry (option handling only; no Gradio).

Run from the repo root:  python -m unittest tts_webui.test_core_server
"""

import unittest
from unittest import mock

from tts_webui import core_server


class UpgradeOptionsTest(unittest.TestCase):
    def test_auth_string_becomes_tuple(self):
        out = core_server.upgrade_gradio_options({"auth": "user:pass"})
        self.assertEqual(out["auth"], ("user", "pass"))

    def test_auth_list_becomes_tuple(self):
        out = core_server.upgrade_gradio_options({"auth": ["u", "p"]})
        self.assertEqual(out["auth"], ("u", "p"))

    def test_auth_none_stays(self):
        self.assertIsNone(core_server.upgrade_gradio_options({"auth": None})["auth"])

    def test_missing_auth_is_ok(self):
        self.assertEqual(core_server.upgrade_gradio_options({"a": 1}), {"a": 1})

    def test_drops_legacy_keys(self):
        opts = {k: 1 for k in core_server._DROPPED_OPTIONS}
        opts["server_port"] = 7860
        self.assertEqual(core_server.upgrade_gradio_options(opts), {"server_port": 7860})


class LaunchOptionsTest(unittest.TestCase):
    def test_launch_kwargs(self):
        kw = core_server.launch_options(
            {"server_port": 4321, "server_name": "127.0.0.1", "inbrowser": True, "auth": None}
        )
        self.assertEqual(kw["server_port"], 4321)
        self.assertEqual(kw["server_name"], "127.0.0.1")
        self.assertIs(kw["inbrowser"], False)
        self.assertEqual(kw["allowed_paths"], ["."])
        self.assertEqual(kw["favicon_path"], "./react-ui/public/favicon.ico")
        self.assertIs(kw["prevent_thread_lock"], True)


    def test_overrides_apply_to_a_copy_and_config_keeps_public_port(self):
        config_opts = {"server_port": 7770, "server_name": "0.0.0.0", "inbrowser": True, "auth": "u:p"}
        kw = core_server.launch_options(
            config_opts, {"server_port": 27700, "server_name": "127.0.0.1"}
        )
        self.assertEqual((kw["server_port"], kw["server_name"]), (27700, "127.0.0.1"))
        self.assertEqual(
            config_opts,
            {"server_port": 7770, "server_name": "0.0.0.0", "inbrowser": True, "auth": "u:p"},
        )


class ServerCoreTest(unittest.TestCase):
    def test_server_core_runs_thin_entry_not_server_py(self):
        import server_core

        with mock.patch("tts_webui.core_router_bridge.install"), mock.patch(
            "tts_webui.core_server.main"
        ) as run, mock.patch("runpy.run_path") as runpy_run:
            server_core.main()
        run.assert_called_once()
        runpy_run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
