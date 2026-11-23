"""unittest for the thin Go-core entry (no real Gradio).

Which options Gradio is launched with is decided in Go
(core/internal/serverpy/launchopts); this tests that Python only passes them on.

Run from the repo root:  python -m unittest tts_webui.test_core_server
"""

import sys
import types
import unittest
from unittest import mock

from tts_webui import core_server


class LaunchKwargsTest(unittest.TestCase):
    def test_auth_pairs_become_tuples(self):
        out = core_server.launch_kwargs({"auth": [["u", "p"], ["v", "q"]], "share": False})
        self.assertEqual(out, {"auth": [("u", "p"), ("v", "q")], "share": False})

    def test_no_auth_stays(self):
        self.assertEqual(core_server.launch_kwargs({"auth": None, "a": 1}), {"auth": None, "a": 1})
        self.assertEqual(core_server.launch_kwargs({}), {})

    def test_the_answer_is_not_mutated(self):
        answer = {"auth": [["u", "p"]]}
        core_server.launch_kwargs(answer)
        self.assertEqual(answer, {"auth": [["u", "p"]]})


class MainTest(unittest.TestCase):
    def test_launches_with_exactly_what_go_answered_and_prints_its_lines(self):
        answer = {
            "options": {
                "server_name": "127.0.0.1",
                "server_port": 27700,
                "auth": [["u", "p"]],
                "prevent_thread_lock": True,
            },
            "lines": ["Gradio server authentication enabled", "Starting Gradio server: http://127.0.0.1:27700"],
        }
        demo = mock.MagicMock()
        modules = {
            "tts_webui.utils.suppress_warnings": types.SimpleNamespace(suppress_warnings=mock.Mock()),
            "tts_webui.utils.torch_load_patch": types.SimpleNamespace(apply_torch_load_patch=mock.Mock()),
            "tts_webui.config.config": types.SimpleNamespace(config={"x": 1}),
            "tts_webui.gradio.blocks": types.SimpleNamespace(main_block=mock.Mock(return_value=demo)),
        }
        launch = mock.Mock(return_value=answer)
        fake_bridge = types.SimpleNamespace(launch=types.SimpleNamespace(options=launch))
        with mock.patch.dict(sys.modules, modules), mock.patch(
            "tts_webui.core_bridge.bridge", fake_bridge
        ), mock.patch("builtins.print") as out:
            core_server.main()
        launch.assert_called_once_with()
        modules["tts_webui.gradio.blocks"].main_block.assert_called_once_with(config={"x": 1})
        demo.queue.return_value.launch.assert_called_once_with(
            server_name="127.0.0.1",
            server_port=27700,
            auth=[("u", "p")],
            prevent_thread_lock=True,
        )
        printed = [c.args[0] for c in out.call_args_list if c.args]
        self.assertEqual(printed[1:], answer["lines"])  # after "Starting TTS WebUI... "

    def test_the_module_has_no_option_logic_left(self):
        with open(core_server.__file__, encoding="utf-8") as f:
            src = f.read()
        for gone in ("allowed_paths", "favicon", "inbrowser", "prevent_thread_lock", "server_port", "dotenv", "os.environ"):
            self.assertNotIn(gone, src)


class ServerCoreTest(unittest.TestCase):
    def test_server_core_installs_the_config_bridge_and_runs_the_thin_entry(self):
        import server_core

        with (
            mock.patch("tts_webui.core_config_bridge.install") as install_config,
            mock.patch("tts_webui.core_server.main") as run,
        ):
            server_core.main()
        install_config.assert_called_once()
        run.assert_called_once()


if __name__ == "__main__":
    unittest.main()
