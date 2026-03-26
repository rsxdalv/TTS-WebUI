"""
Tests for MiniMax Cloud TTS extension.
"""

import sys
from pathlib import Path

# Ensure project root is on sys.path so 'extensions' is importable
# This must be before any project imports
project_root = Path(__file__).parent.parent
sys.path.insert(0, str(project_root))

import json
import os
from unittest.mock import MagicMock, patch

import pytest

from extensions.builtin.extension_minimax_cloud_tts.main import (
    MINIMAX_API_URL,
    MINIMAX_MODELS,
    MINIMAX_VOICES,
    _get_api_key,
    _save_audio,
    extension__tts_generation_webui,
    generate_minimax_tts,
)


# ---------------------------------------------------------------------------
# Unit Tests
# ---------------------------------------------------------------------------
class TestConstants:
    """Test that constants are correctly defined."""

    @pytest.mark.unit
    def test_api_url(self):
        assert MINIMAX_API_URL == "https://api.minimax.io/v1/t2a_v2"

    @pytest.mark.unit
    def test_models_not_empty(self):
        assert len(MINIMAX_MODELS) >= 2

    @pytest.mark.unit
    def test_models_contain_hd(self):
        assert "speech-2.8-hd" in MINIMAX_MODELS

    @pytest.mark.unit
    def test_models_contain_turbo(self):
        assert "speech-2.8-turbo" in MINIMAX_MODELS

    @pytest.mark.unit
    def test_voices_not_empty(self):
        assert len(MINIMAX_VOICES) >= 10

    @pytest.mark.unit
    def test_voices_contain_graceful_lady(self):
        assert "English_Graceful_Lady" in MINIMAX_VOICES

    @pytest.mark.unit
    def test_voices_contain_deep_voice_man(self):
        assert "Deep_Voice_Man" in MINIMAX_VOICES


class TestGetApiKey:
    """Test API key resolution."""

    @pytest.mark.unit
    def test_explicit_key_takes_priority(self):
        os.environ["MINIMAX_API_KEY"] = "env_key"
        result = _get_api_key("explicit_key")
        assert result == "explicit_key"

    @pytest.mark.unit
    def test_env_key_fallback(self):
        os.environ["MINIMAX_API_KEY"] = "env_key"
        result = _get_api_key("")
        assert result == "env_key"

    @pytest.mark.unit
    def test_empty_when_no_key(self):
        os.environ.pop("MINIMAX_API_KEY", None)
        result = _get_api_key("")
        assert result == ""

    @pytest.mark.unit
    def test_whitespace_stripped(self):
        result = _get_api_key("  my_key  ")
        assert result == "my_key"

    @pytest.mark.unit
    def test_none_like_empty(self):
        os.environ.pop("MINIMAX_API_KEY", None)
        result = _get_api_key("")
        assert result == ""


class TestSaveAudio:
    """Test audio saving functionality."""

    @pytest.mark.unit
    def test_save_audio_creates_files(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_hex = b"fake audio data".hex()
        audio_path, metadata, folder_root = _save_audio(
            audio_hex, "Hello world", "speech-2.8-hd", "English_Graceful_Lady"
        )

        assert os.path.exists(audio_path)
        assert audio_path.endswith(".mp3")
        assert os.path.exists(folder_root)

        with open(audio_path, "rb") as f:
            assert f.read() == b"fake audio data"

    @pytest.mark.unit
    def test_save_audio_metadata(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_hex = b"test".hex()
        audio_path, metadata, folder_root = _save_audio(
            audio_hex, "Test text", "speech-2.8-turbo", "cute_boy"
        )

        assert metadata["text"] == "Test text"
        assert metadata["model"] == "speech-2.8-turbo"
        assert metadata["voice_id"] == "cute_boy"
        assert metadata["provider"] == "minimax"

    @pytest.mark.unit
    def test_save_audio_json_file(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_hex = b"test".hex()
        audio_path, metadata, folder_root = _save_audio(
            audio_hex, "Test", "speech-2.8-hd", "Wise_Woman"
        )

        json_path = audio_path.replace(".mp3", ".json")
        assert os.path.exists(json_path)
        with open(json_path) as f:
            saved_meta = json.load(f)
        assert saved_meta["provider"] == "minimax"
        assert saved_meta["voice_id"] == "Wise_Woman"

    @pytest.mark.unit
    def test_save_audio_special_chars_in_text(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_hex = b"test".hex()
        audio_path, metadata, folder_root = _save_audio(
            audio_hex, "Hello! @#$% world", "speech-2.8-hd", "cute_boy"
        )

        assert os.path.exists(audio_path)
        assert "@" not in audio_path
        assert "#" not in audio_path

    @pytest.mark.unit
    def test_save_audio_empty_text(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_hex = b"test".hex()
        audio_path, metadata, folder_root = _save_audio(
            audio_hex, "", "speech-2.8-hd", "cute_boy"
        )

        assert os.path.exists(audio_path)
        assert "minimax_tts" in audio_path


class TestGenerateMiniMaxTTS:
    """Test the main generation function."""

    @pytest.mark.unit
    def test_no_api_key_raises_error(self):
        os.environ.pop("MINIMAX_API_KEY", None)
        # gradio.Error is a subclass of Exception
        with pytest.raises(Exception, match="API key"):
            generate_minimax_tts("Hello", "speech-2.8-hd", "English_Graceful_Lady", "")

    @pytest.mark.unit
    def test_empty_text_raises_error(self):
        os.environ["MINIMAX_API_KEY"] = "test_key"
        with pytest.raises(Exception, match="text"):
            generate_minimax_tts("", "speech-2.8-hd", "English_Graceful_Lady", "")

    @pytest.mark.unit
    def test_whitespace_text_raises_error(self):
        os.environ["MINIMAX_API_KEY"] = "test_key"
        with pytest.raises(Exception, match="text"):
            generate_minimax_tts("   ", "speech-2.8-hd", "English_Graceful_Lady", "")

    @pytest.mark.unit
    def test_successful_generation(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        fake_audio_hex = b"fake mp3 audio content here".hex()
        mock_response = json.dumps(
            {
                "base_resp": {"status_code": 0, "status_msg": "success"},
                "data": {"audio": fake_audio_hex},
            }
        ).encode("utf-8")

        mock_resp = MagicMock()
        mock_resp.read.return_value = mock_response
        mock_resp.__enter__ = MagicMock(return_value=mock_resp)
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch("urllib.request.urlopen", return_value=mock_resp):
            audio_path, metadata, folder_root = generate_minimax_tts(
                "Hello world",
                "speech-2.8-hd",
                "English_Graceful_Lady",
                "test_api_key",
            )

        assert os.path.exists(audio_path)
        assert metadata["text"] == "Hello world"
        assert metadata["model"] == "speech-2.8-hd"
        assert metadata["voice_id"] == "English_Graceful_Lady"

    @pytest.mark.unit
    def test_api_error_response(self):
        os.environ["MINIMAX_API_KEY"] = "test_key"

        mock_response = json.dumps(
            {
                "base_resp": {"status_code": 1001, "status_msg": "Invalid API key"},
            }
        ).encode("utf-8")

        mock_resp = MagicMock()
        mock_resp.read.return_value = mock_response
        mock_resp.__enter__ = MagicMock(return_value=mock_resp)
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch("urllib.request.urlopen", return_value=mock_resp):
            with pytest.raises(Exception, match="Invalid API key"):
                generate_minimax_tts(
                    "Hello", "speech-2.8-hd", "English_Graceful_Lady", ""
                )

    @pytest.mark.unit
    def test_http_error(self):
        import urllib.error

        os.environ["MINIMAX_API_KEY"] = "test_key"

        mock_error = urllib.error.HTTPError(
            url="https://api.minimax.io/v1/t2a_v2",
            code=401,
            msg="Unauthorized",
            hdrs=None,
            fp=MagicMock(read=MagicMock(return_value=b"Unauthorized")),
        )

        with patch("urllib.request.urlopen", side_effect=mock_error):
            with pytest.raises(Exception, match="401"):
                generate_minimax_tts(
                    "Hello", "speech-2.8-hd", "English_Graceful_Lady", ""
                )

    @pytest.mark.unit
    def test_request_payload_structure(self, tmp_path, monkeypatch):
        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        fake_audio_hex = b"test".hex()
        mock_response = json.dumps(
            {
                "data": {"audio": fake_audio_hex},
            }
        ).encode("utf-8")

        mock_resp = MagicMock()
        mock_resp.read.return_value = mock_response
        mock_resp.__enter__ = MagicMock(return_value=mock_resp)
        mock_resp.__exit__ = MagicMock(return_value=False)

        captured_request = {}

        def capture_request(req, **kwargs):
            captured_request["url"] = req.full_url
            captured_request["data"] = json.loads(req.data.decode("utf-8"))
            captured_request["headers"] = dict(req.headers)
            return mock_resp

        with patch("urllib.request.urlopen", side_effect=capture_request):
            generate_minimax_tts(
                "Test text",
                "speech-2.8-turbo",
                "Deep_Voice_Man",
                "my_api_key",
            )

        assert captured_request["url"] == MINIMAX_API_URL
        assert captured_request["data"]["model"] == "speech-2.8-turbo"
        assert captured_request["data"]["text"] == "Test text"
        assert captured_request["data"]["voice_setting"]["voice_id"] == "Deep_Voice_Man"
        assert captured_request["data"]["audio_setting"]["format"] == "mp3"
        assert "Bearer my_api_key" in captured_request["headers"].get(
            "Authorization", ""
        )


class TestExtensionMetadata:
    """Test extension entry point metadata."""

    @pytest.mark.unit
    def test_extension_function_exists(self):
        assert callable(extension__tts_generation_webui)

    @pytest.mark.unit
    def test_metadata_fields(self):
        # We can't call the function (requires gradio context),
        # but we can import and check the module structure
        from extensions.builtin.extension_minimax_cloud_tts import main

        assert hasattr(main, "extension__tts_generation_webui")
        assert hasattr(main, "generate_minimax_tts")
        assert hasattr(main, "minimax_cloud_tts_tab")
        assert hasattr(main, "MINIMAX_MODELS")
        assert hasattr(main, "MINIMAX_VOICES")


# ---------------------------------------------------------------------------
# Integration Tests
# ---------------------------------------------------------------------------
class TestMiniMaxTTSIntegration:
    """Integration tests for the full generation pipeline."""

    @pytest.mark.integration
    @pytest.mark.requires_network
    def test_real_api_call(self, tmp_path, monkeypatch):
        """Test with real MiniMax API. Requires MINIMAX_API_KEY env var."""
        api_key = os.environ.get("MINIMAX_API_KEY", "")
        if not api_key:
            pytest.skip("MINIMAX_API_KEY not set")

        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_path, metadata, folder_root = generate_minimax_tts(
            "Hello, this is a test of MiniMax Cloud TTS.",
            "speech-2.8-turbo",
            "English_Graceful_Lady",
            api_key,
        )

        assert os.path.exists(audio_path)
        assert os.path.getsize(audio_path) > 0
        assert metadata["provider"] == "minimax"
        assert metadata["model"] == "speech-2.8-turbo"

        json_path = audio_path.replace(".mp3", ".json")
        assert os.path.exists(json_path)

    @pytest.mark.integration
    @pytest.mark.requires_network
    def test_hd_model(self, tmp_path, monkeypatch):
        """Test with speech-2.8-hd model."""
        api_key = os.environ.get("MINIMAX_API_KEY", "")
        if not api_key:
            pytest.skip("MINIMAX_API_KEY not set")

        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        audio_path, metadata, folder_root = generate_minimax_tts(
            "Testing high-definition speech model.",
            "speech-2.8-hd",
            "Deep_Voice_Man",
            api_key,
        )

        assert os.path.exists(audio_path)
        assert os.path.getsize(audio_path) > 0
        assert metadata["model"] == "speech-2.8-hd"
        assert metadata["voice_id"] == "Deep_Voice_Man"

    @pytest.mark.integration
    @pytest.mark.requires_network
    def test_multiple_voices(self, tmp_path, monkeypatch):
        """Test that different voices produce output."""
        api_key = os.environ.get("MINIMAX_API_KEY", "")
        if not api_key:
            pytest.skip("MINIMAX_API_KEY not set")

        monkeypatch.chdir(tmp_path)
        os.makedirs(tmp_path / "outputs", exist_ok=True)

        for voice_id in ["English_Graceful_Lady", "cute_boy", "Deep_Voice_Man"]:
            audio_path, metadata, folder_root = generate_minimax_tts(
                "Voice test.",
                "speech-2.8-turbo",
                voice_id,
                api_key,
            )
            assert os.path.exists(audio_path)
            assert metadata["voice_id"] == voice_id
