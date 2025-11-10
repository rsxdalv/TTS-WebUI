"""
Tests for the extensions data loader module.
"""

import json
import os
import sys
import unittest
from unittest.mock import mock_open, patch

# Add the project root directory to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from tts_webui.extensions_loader.extensions_data_loader import (
    DEFAULT_EXTENSIONS_FILE,
    EXTERNAL_EXTENSIONS_FILE,
    _flatten_interface_tabs,
    create_empty_external_extensions_file,
    filter_extensions_by_type_and_class,
    get_decorator_extensions,
    get_decorator_extensions_by_class,
    get_extension_example,
    get_interface_extensions,
    get_interface_extensions_by_class,
    load_extensions_json,
    load_json_file,
    load_merged_extensions_data,
    merge_extensions_data,
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
            "example_extension": {
                "package_name": "example_extension",
                "name": "Example Extension",
            },
        }

        self.external_extensions = {
            "tabs": [
                {
                    "package_name": "extension_external1",
                    "name": "External Extension 1",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                },
                {
                    "package_name": "extension_base1",  # Duplicate, should be ignored
                    "name": "Base Extension 1 (External)",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                },
            ],
            "decorators": [
                {
                    "package_name": "decorator_external1",
                    "name": "Decorator External 1",
                    "extension_type": "decorator",
                    "extension_class": "inner",
                },
            ],
            "tabsInGroups": {
                "groupA": [
                    {
                        "package_name": "extension_group_a1",
                        "name": "Group A 1",
                        "extension_type": "interface",
                        "extension_class": "audio-music-generation",
                    }
                ],
                "groupB": [
                    {
                        "package_name": "extension_group_b1",
                        "name": "Group B 1",
                        "extension_type": "interface",
                        "extension_class": "text-to-speech",
                    },
                    {
                        "package_name": "extension_base2",  # duplicate from base tabs
                        "name": "Base Extension 2 (Dup)",
                        "extension_type": "interface",
                        "extension_class": "audio-music-generation",
                    },
                ],
            },
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

    def test_merge_extensions_data(self):
        """Test merging extensions data."""
        result = merge_extensions_data(self.base_extensions, self.external_extensions)

        # Only decorators are concatenated; tabs are unchanged in merge
        self.assertEqual(len(result.get("tabs", [])), 2)
        self.assertEqual(len(result["decorators"]), 2)  # 1 base + 1 external

    @patch("os.path.exists")
    @patch("tts_webui.extensions_loader.extensions_data_loader.load_json_file")
    def test_load_merged_extensions_data_with_external(
        self, mock_load_json, mock_exists
    ):
        """Test loading and merging extensions data with external file."""
        mock_exists.return_value = True
        mock_load_json.side_effect = [self.base_extensions, self.external_extensions]

        result = load_merged_extensions_data()

        # Verify both files were loaded
        self.assertEqual(mock_load_json.call_count, 2)
        mock_load_json.assert_any_call(DEFAULT_EXTENSIONS_FILE)
        mock_load_json.assert_any_call(EXTERNAL_EXTENSIONS_FILE)

        # Check that the merge happened correctly (only decorators concatenated)
        self.assertEqual(len(result.get("tabs", [])), 2)
        self.assertEqual(len(result["decorators"]), 2)  # 1 base + 1 external

    @patch("os.path.exists")
    @patch("tts_webui.extensions_loader.extensions_data_loader.load_json_file")
    def test_load_merged_extensions_data_without_external(
        self, mock_load_json, mock_exists
    ):
        """Test loading extensions data without external file."""
        mock_exists.return_value = False
        mock_load_json.return_value = self.base_extensions

        result = load_merged_extensions_data()

        # Verify only the base file was loaded
        mock_load_json.assert_called_once_with(DEFAULT_EXTENSIONS_FILE)

        # Check that the result is the base extensions
        self.assertEqual(result, self.base_extensions)

    @patch(
        "tts_webui.extensions_loader.extensions_data_loader.load_merged_extensions_data"
    )
    def test_get_decorator_extensions(self, mock_load_merged):
        """Test getting decorator extensions."""
        mock_load_merged.return_value = self.base_extensions
        result = get_decorator_extensions()
        self.assertEqual(result, self.base_extensions["decorators"])

    @patch("os.path.exists")
    @patch("tts_webui.extensions_loader.extensions_data_loader.load_extensions_json")
    def test_get_interface_extensions(self, mock_load_base, mock_exists):
        """Test getting interface extensions."""
        mock_exists.return_value = False
        mock_load_base.return_value = self.base_extensions
        result = get_interface_extensions()
        self.assertEqual(result, self.base_extensions["tabs"])

    @patch(
        "tts_webui.extensions_loader.extensions_data_loader.load_merged_extensions_data"
    )
    def test_get_extension_example(self, mock_load_merged):
        """Test getting the example extension template."""
        mock_load_merged.return_value = self.base_extensions
        result = get_extension_example()
        self.assertEqual(result, self.base_extensions["example_extension"])

    def test_filter_extensions_by_type_and_class(self):
        """Test filtering extensions by type and class."""
        extensions = self.base_extensions["tabs"] + self.base_extensions["decorators"]

        # Filter by type only
        result = filter_extensions_by_type_and_class(extensions, "interface")
        self.assertEqual(len(result), 2)
        self.assertTrue(all(x["extension_type"] == "interface" for x in result))

        # Filter by type and class
        result = filter_extensions_by_type_and_class(
            extensions, "interface", "text-to-speech"
        )
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["package_name"], "extension_base1")

    @patch(
        "tts_webui.extensions_loader.extensions_data_loader.get_decorator_extensions"
    )
    def test_get_decorator_extensions_by_class(self, mock_get_decorators):
        """Test getting decorator extensions by class."""
        mock_get_decorators.return_value = (
            self.base_extensions["decorators"] + self.external_extensions["decorators"]
        )

        result = get_decorator_extensions_by_class("outer")
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["package_name"], "decorator_base1")

    @patch(
        "tts_webui.extensions_loader.extensions_data_loader.get_interface_extensions"
    )
    def test_get_interface_extensions_by_class(self, mock_get_interfaces):
        """Test getting interface extensions by class."""
        mock_get_interfaces.return_value = (
            self.base_extensions["tabs"] + self.external_extensions["tabs"]
        )

        result = get_interface_extensions_by_class("audio-music-generation")
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["package_name"], "extension_base2")

    @patch("os.path.exists")
    @patch("builtins.open", new_callable=mock_open)
    def test_create_empty_external_extensions_file(self, mock_file, mock_exists):
        """Test creating an empty external extensions file."""
        # Test when file doesn't exist
        mock_exists.return_value = False
        result = create_empty_external_extensions_file()

        self.assertTrue(result)
        mock_file.assert_called_once_with(EXTERNAL_EXTENSIONS_FILE, "w")
        mock_file().write.assert_called()  # json.dump calls write

        # Test when file already exists
        mock_exists.return_value = True
        mock_file.reset_mock()

        result = create_empty_external_extensions_file()
        self.assertFalse(result)
        mock_file.assert_not_called()

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
