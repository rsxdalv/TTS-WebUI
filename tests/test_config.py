"""
Unit tests for tts_webui.config module.
"""

import json
import os
import pytest
from pathlib import Path
from unittest.mock import patch, mock_open

from tts_webui.config.config import config
from tts_webui.config.load_config import load_config, default_config
from tts_webui.config.config_utils import get_config_value, set_config_value
from tts_webui.config._save_config import _save_json, _save_config
from tts_webui.config.save_config_gradio import (
    _convert_empty_strings_to_none,
    _recreate_ui_dict,
    save_config_gradio,
)


class TestLoadConfig:
    """Tests for config loading functionality."""

    @pytest.mark.unit
    def test_load_config_creates_default_when_missing(self, change_to_temp_dir):
        """Test that load_config creates a default config when file doesn't exist."""
        assert not (Path.cwd() / "config.json").exists()
        
        result = load_config()
        
        assert (Path.cwd() / "config.json").exists()
        assert result == default_config
        assert "model" in result
        assert "gradio_interface_options" in result
        assert "extensions" in result

    @pytest.mark.unit
    def test_load_config_reads_existing_file(self, temp_config_file):
        """Test that load_config reads an existing config file."""
        os.chdir(temp_config_file.parent)
        
        result = load_config()
        
        assert result is not None
        assert "model" in result
        assert "gradio_interface_options" in result
        assert result["gradio_interface_options"]["server_port"] == 7770

    @pytest.mark.unit
    def test_default_config_structure(self):
        """Test that default config has expected structure."""
        assert "model" in default_config
        assert "gradio_interface_options" in default_config
        assert "extensions" in default_config
        
        # Check model config
        assert "text_use_gpu" in default_config["model"]
        assert isinstance(default_config["model"]["text_use_gpu"], bool)
        
        # Check gradio options
        assert "server_port" in default_config["gradio_interface_options"]
        assert "server_name" in default_config["gradio_interface_options"]
        
        # Check extensions
        assert "disabled" in default_config["extensions"]
        assert isinstance(default_config["extensions"]["disabled"], list)


class TestConfigUtils:
    """Tests for config utility functions."""

    @pytest.mark.unit
    def test_get_config_value_existing_key(self, mock_config):
        """Test getting a value that exists in config."""
        with patch("tts_webui.config.config_utils.config", mock_config):
            result = get_config_value("model", "text_use_gpu")
            assert result is False

    @pytest.mark.unit
    def test_get_config_value_missing_key_returns_default(self, mock_config):
        """Test getting a value that doesn't exist returns default."""
        with patch("tts_webui.config.config_utils.config", mock_config):
            result = get_config_value("model", "nonexistent_key", default="default_value")
            assert result == "default_value"

    @pytest.mark.unit
    def test_get_config_value_missing_namespace(self, mock_config):
        """Test getting a value from missing namespace returns default."""
        with patch("tts_webui.config.config_utils.config", mock_config):
            result = get_config_value("nonexistent_namespace", "key", default="default")
            assert result == "default"

    @pytest.mark.unit
    def test_set_config_value(self, mock_config):
        """Test setting a config value."""
        with patch("tts_webui.config.config_utils.config", mock_config):
            set_config_value("model", "text_use_gpu", True)
            assert mock_config["model"]["text_use_gpu"] is True


class TestSaveConfig:
    """Tests for config saving functionality."""

    @pytest.mark.unit
    def test_save_json(self, temp_dir):
        """Test saving JSON data to file."""
        test_data = {"test": "data", "number": 42}
        file_path = temp_dir / "test_config.json"
        
        _save_json(str(file_path), test_data)
        
        assert file_path.exists()
        with open(file_path) as f:
            loaded_data = json.load(f)
        assert loaded_data == test_data

    @pytest.mark.unit
    def test_save_config(self, change_to_temp_dir):
        """Test saving config to config.json."""
        test_data = {"test_config": {"value": 123}}
        
        _save_config(test_data)
        
        config_path = Path.cwd() / "config.json"
        assert config_path.exists()
        with open(config_path) as f:
            loaded_data = json.load(f)
        assert loaded_data == test_data

    @pytest.mark.unit
    def test_convert_empty_strings_to_none(self):
        """Test converting empty strings to None in dict."""
        test_dict = {
            "key1": "value",
            "key2": "",
            "key3": None,
            "key4": 0,
        }
        
        # Function modifies in place and returns None
        _convert_empty_strings_to_none(test_dict)
        
        assert test_dict["key1"] == "value"
        assert test_dict["key2"] is None
        assert test_dict["key3"] is None
        assert test_dict["key4"] == 0

    @pytest.mark.unit
    def test_recreate_ui_dict(self):
        """Test recreating UI dict from keys and inputs."""
        keys = ["option1", "option2", "option3"]
        inputs = ["value1", "value2", "value3"]
        
        result = _recreate_ui_dict(keys, inputs)
        
        # Function creates a flat dict mapping keys to values
        assert result == {"option1": "value1", "option2": "value2", "option3": "value3"}

    @pytest.mark.unit
    def test_save_config_gradio(self, change_to_temp_dir):
        """Test saving Gradio config."""
        keys = ["server_port", "server_name", "inbrowser"]
        inputs = [7770, "127.0.0.1", True]
        
        # Need to create a basic config first
        with open("config.json", "w") as f:
            json.dump({"gradio_interface_options": {}}, f)
        
        save_config_gradio(keys, inputs)
        
        with open("config.json") as f:
            result = json.load(f)
        
        assert "gradio_interface_options" in result
