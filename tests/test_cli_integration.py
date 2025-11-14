"""
Integration tests for tts_webui CLI.
"""

import os
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest
from typer.testing import CliRunner

from tts_webui.cli import app, extension_list, serve, troubleshoot

runner = CliRunner()


class TestCLI:
    """Tests for CLI commands."""

    @pytest.mark.integration
    def test_cli_app_exists(self):
        """Test that the CLI app is properly initialized."""
        assert app is not None
        assert hasattr(app, "command")

    @pytest.mark.integration
    def test_troubleshoot_command(self, change_to_temp_dir):
        """Test troubleshoot command."""
        # Create minimal requirements.txt
        (Path.cwd() / "requirements.txt").write_text("pytest\ngradio")
        (Path.cwd() / "server.py").write_text("# Server file")

        result = runner.invoke(app, ["troubleshoot"])

        assert "Python executable:" in result.stdout
        assert "Python version:" in result.stdout

    @pytest.mark.integration
    def test_troubleshoot_missing_files(self, change_to_temp_dir):
        """Test troubleshoot with missing files."""
        result = runner.invoke(app, ["troubleshoot"])

        # Should show warnings but not crash
        assert result.exit_code in [0, 1]
        assert "Python executable:" in result.stdout

    @pytest.mark.integration
    def test_extension_list_empty(self, change_to_temp_dir):
        """Test extension list with no extensions."""
        ext_dir = Path.cwd() / "extensions"
        ext_dir.mkdir(exist_ok=True)

        result = runner.invoke(app, ["extension", "list"])

        assert result.exit_code == 0
        assert "No extensions found" in result.stdout

    @pytest.mark.integration
    def test_extension_list_with_extensions(self, change_to_temp_dir):
        """Test extension list with extensions."""
        ext_dir = Path.cwd() / "extensions"
        ext_dir.mkdir(exist_ok=True)

        # Create some mock extensions
        (ext_dir / "extension1").mkdir()
        (ext_dir / "extension2").mkdir()

        result = runner.invoke(app, ["extension", "list"])

        assert result.exit_code == 0
        assert "extension1" in result.stdout
        assert "extension2" in result.stdout

    @pytest.mark.integration
    def test_serve_command_missing_server(self, change_to_temp_dir):
        """Test serve command when server.py is missing."""
        result = runner.invoke(app, ["serve"])

        # Should exit with error
        assert result.exit_code == 2
        assert "server.py not found" in result.stdout
