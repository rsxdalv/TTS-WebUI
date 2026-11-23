"""
Tests for the extensions data loader module.
"""

import json
import unittest
from unittest.mock import mock_open, patch

from tts_webui.extensions_loader.extensions_data_loader import (
    DEFAULT_EXTENSIONS_FILE,
    _flatten_interface_tabs,
    get_interface_extensions,
    load_extensions_json,
    load_json_file,
)


class TestExtensionsDataLoader(unittest.TestCase):
    """Test cases for the extensions data loader module."""

    def setUp(self):
        """Set up test fixtures."""
        self.base_extensions = {
            "tabs": [
                {
                    "package_name": "extension_base1",
                    "name": "Base Extension 1",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                },
                {
                    "package_name": "extension_base2",
                    "name": "Base Extension 2",
                    "extension_type": "interface",
                    "extension_class": "audio-music-generation",
                },
            ],
            "decorators": [
                {
                    "package_name": "decorator_base1",
                    "name": "Decorator Base 1",
                    "extension_type": "decorator",
                    "extension_class": "outer",
                },
            ],
        }

    @patch("builtins.open", new_callable=mock_open, read_data='{"key": "value"}')
    def test_load_json_file(self, mock_file):
        """Test loading a JSON file."""
        result = load_json_file("test.json")
        self.assertEqual(result, {"key": "value"})
        mock_file.assert_called_once_with("test.json", "r")

    @patch("builtins.open", side_effect=Exception("File not found"))
    def test_load_json_file_error(self, mock_file):
        """Test loading a JSON file that doesn't exist."""
        result = load_json_file("nonexistent.json")
        self.assertEqual(result, {})

    @patch("tts_webui.extensions_loader.extensions_data_loader.load_json_file")
    def test_load_extensions_json(self, mock_load_json):
        """Test loading the extensions.json file."""
        mock_load_json.return_value = {"key": "value"}
        result = load_extensions_json()
        self.assertEqual(result, {"key": "value"})
        mock_load_json.assert_called_once_with(DEFAULT_EXTENSIONS_FILE)

    @patch("os.path.exists")
    @patch("tts_webui.extensions_loader.extensions_data_loader.load_extensions_json")
    def test_get_interface_extensions(self, mock_load_base, mock_exists):
        """Test getting interface extensions."""
        mock_exists.return_value = False
        mock_load_base.return_value = self.base_extensions
        result = get_interface_extensions()
        self.assertEqual(result, self.base_extensions["tabs"])

    @patch("os.path.exists")
    @patch("tts_webui.extensions_loader.extensions_data_loader.load_extensions_json")
    def test_get_interface_extensions_flattens_groups(
        self, mock_load_base, mock_exists
    ):
        """Test get_interface_extensions flattens tabsInGroups and deduplicates by package_name."""
        mock_exists.return_value = False
        # deep copy of base and add grouped tabs including a duplicate
        merged = json.loads(json.dumps(self.base_extensions))
        merged["tabsInGroups"] = {
            "g1": [
                {
                    "package_name": "extension_group_1",
                    "name": "Group 1",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                },
                # duplicate of base first tab should not add a second copy
                self.base_extensions["tabs"][0],
            ]
        }
        mock_load_base.return_value = merged
        result = get_interface_extensions()
        pkgs = [x.get("package_name") for x in result]
        self.assertIn("extension_group_1", pkgs)
        self.assertEqual(pkgs.count(self.base_extensions["tabs"][0]["package_name"]), 1)

    def test_flatten_interface_tabs_helper(self):
        data = {
            "tabs": [
                {
                    "package_name": "a",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                }
            ],
            "tabsInGroups": {
                "g1": [
                    {
                        "package_name": "b",
                        "extension_type": "interface",
                        "extension_class": "text-to-speech",
                    },
                    {
                        "package_name": "a",
                        "extension_type": "interface",
                        "extension_class": "text-to-speech",
                    },
                ],
                "g2": [
                    {
                        "package_name": "c",
                        "extension_type": "interface",
                        "extension_class": "audio-music-generation",
                    }
                ],
            },
        }
        flattened = _flatten_interface_tabs(data)
        pkgs = [x.get("package_name") for x in flattened]
        self.assertEqual(pkgs, ["a", "b", "c"])  # order: tabs then groups, dedup


if __name__ == "__main__":
    unittest.main()
