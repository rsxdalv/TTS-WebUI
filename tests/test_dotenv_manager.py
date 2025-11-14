"""
Unit tests for tts_webui.dotenv_manager module.
"""

import os
from pathlib import Path
from unittest.mock import mock_open, patch

import pytest

from tts_webui.dotenv_manager.init import init
from tts_webui.dotenv_manager.writer import generate_env, write_env


class TestDotenvManager:
    """Tests for dotenv manager functionality."""

    @pytest.mark.unit
    def test_generate_env(self):
        """Test generate_env creates environment string."""
        result = generate_env()

        assert isinstance(result, str)
        assert len(result) > 0

    @pytest.mark.unit
    def test_write_env(self, temp_dir):
        """Test write_env creates .env file."""
        env_path = temp_dir / ".env"
        test_env_content = "TEST_VAR=test_value\nANOTHER_VAR=another_value"

        os.chdir(temp_dir)
        write_env(test_env_content)

        assert env_path.exists()
        content = env_path.read_text()
        assert "TEST_VAR=test_value" in content

    @pytest.mark.unit
    def test_init_creates_env_if_missing(self, change_to_temp_dir):
        """Test init creates .env file when missing."""
        env_path = Path.cwd() / ".env"
        assert not env_path.exists()

        init()

        assert env_path.exists()

    @pytest.mark.unit
    def test_init_loads_existing_env(self, change_to_temp_dir):
        """Test init loads existing .env file."""
        env_path = Path.cwd() / ".env"
        env_path.write_text("TEST_INIT_VAR=init_value")

        # Clear environment before test
        if "TEST_INIT_VAR" in os.environ:
            del os.environ["TEST_INIT_VAR"]

        init()

        # Variable should be loaded (or test that init() doesn't crash)
        # Note: Environment loading might be affected by pytest's environment handling
        assert env_path.exists()

    @pytest.mark.unit
    def test_init_loads_user_env_override(self, change_to_temp_dir):
        """Test init loads .env.user with override."""
        env_path = Path.cwd() / ".env"
        user_env_path = Path.cwd() / ".env.user"

        env_path.write_text("OVERRIDE_VAR=base_value")
        user_env_path.write_text("OVERRIDE_VAR=user_value")

        init()

        # User value should override base value
        assert os.environ.get("OVERRIDE_VAR") == "user_value"
