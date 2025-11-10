"""
Pytest configuration and fixtures for tts_webui tests.
"""

import json
import os
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any, Dict, Generator

import pytest

# Add project root to Python path
project_root = Path(__file__).parent.parent
sys.path.insert(0, str(project_root))


@pytest.fixture
def temp_dir() -> Generator[Path, None, None]:
    """Create a temporary directory for test files."""
    temp_path = Path(tempfile.mkdtemp())
    try:
        yield temp_path
    finally:
        shutil.rmtree(temp_path, ignore_errors=True)


@pytest.fixture
def temp_config_file(temp_dir: Path) -> Generator[Path, None, None]:
    """Create a temporary config.json file."""
    config_path = temp_dir / "config.json"
    config_data = {
        "model": {
            "text_use_gpu": False,
            "text_use_small": True,
        },
        "gradio_interface_options": {
            "server_name": "127.0.0.1",
            "server_port": 7770,
            "inbrowser": False,
            "share": False,
        },
        "extensions": {
            "disabled": [],
        },
    }
    with open(config_path, "w") as f:
        json.dump(config_data, f, indent=2)
    yield config_path


@pytest.fixture
def mock_config() -> Dict[str, Any]:
    """Provide a mock configuration dictionary."""
    return {
        "model": {
            "text_use_gpu": False,
            "text_use_small": True,
            "coarse_use_gpu": False,
            "coarse_use_small": True,
            "fine_use_gpu": False,
            "fine_use_small": True,
            "codec_use_gpu": False,
        },
        "gradio_interface_options": {
            "inline": False,
            "inbrowser": False,
            "share": False,
            "debug": False,
            "max_threads": 40,
            "auth": None,
            "auth_message": None,
            "prevent_thread_lock": False,
            "show_error": False,
            "server_name": "127.0.0.1",
            "server_port": 7770,
            "height": 500,
            "width": "100%",
            "favicon_path": None,
            "ssl_keyfile": None,
            "ssl_certfile": None,
            "ssl_keyfile_password": None,
            "ssl_verify": True,
            "quiet": True,
            "show_api": True,
            "_frontend": True,
        },
        "extensions": {
            "disabled": [],
        },
    }


@pytest.fixture
def mock_extensions_data() -> Dict[str, Any]:
    """Provide mock extensions data."""
    return {
        "tabs": [
            {
                "package_name": "test_extension_1",
                "name": "Test Extension 1",
                "extension_type": "interface",
                "extension_class": "text-to-speech",
            },
        ],
        "decorators": [
            {
                "package_name": "test_decorator_1",
                "name": "Test Decorator 1",
                "extension_type": "decorator",
                "extension_class": "outer",
            },
        ],
    }


@pytest.fixture
def original_cwd():
    """Preserve and restore the original working directory."""
    original = os.getcwd()
    yield original
    os.chdir(original)


@pytest.fixture
def change_to_temp_dir(temp_dir: Path, original_cwd):
    """Change to temporary directory and restore on cleanup."""
    os.chdir(temp_dir)
    yield temp_dir
    os.chdir(original_cwd)


@pytest.fixture(autouse=True)
def reset_environment():
    """Reset environment variables after each test."""
    original_env = os.environ.copy()
    yield
    # Restore original environment
    os.environ.clear()
    os.environ.update(original_env)
