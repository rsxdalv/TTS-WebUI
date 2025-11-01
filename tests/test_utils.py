"""
Unit tests for tts_webui.utils module.
"""

import os
import pytest
import datetime
import numpy as np
from pathlib import Path
from unittest.mock import patch, MagicMock

from tts_webui.utils.date import get_date_string
from tts_webui.utils.create_base_filename import (
    _create_base_filename,
    create_base_filename,
    replace_path_sep,
)
from tts_webui.utils.audio_array_to_sha256 import audio_array_to_sha256
from tts_webui.utils.get_path_from_root import get_path_from_root
from tts_webui.utils.get_dict_props import get_dict_props
from tts_webui.utils.set_seed import set_seed
from tts_webui.utils.randomize_seed import randomize_seed


class TestDateUtils:
    """Tests for date utility functions."""

    @pytest.mark.unit
    def test_get_date_string_format(self):
        """Test that get_date_string returns correctly formatted date."""
        result = get_date_string()
        
        # Should match format: YYYY-MM-DD_HH-MM-SS
        assert len(result) == 19
        assert result[4] == "-"
        assert result[7] == "-"
        assert result[10] == "_"
        assert result[13] == "-"
        assert result[16] == "-"

    @pytest.mark.unit
    def test_get_date_string_parseable(self):
        """Test that get_date_string returns a parseable date."""
        result = get_date_string()
        
        # Should be able to parse it back to datetime
        parsed = datetime.datetime.strptime(result, "%Y-%m-%d_%H-%M-%S")
        assert isinstance(parsed, datetime.datetime)


class TestFilenameUtils:
    """Tests for filename utility functions."""

    @pytest.mark.unit
    def test_replace_path_sep_with_none(self):
        """Test replace_path_sep handles None input."""
        result = replace_path_sep(None)
        assert result == "None"

    @pytest.mark.unit
    def test_replace_path_sep_with_normal_string(self):
        """Test replace_path_sep with normal string."""
        result = replace_path_sep("test_string")
        assert result == "test_string"

    @pytest.mark.unit
    def test_replace_path_sep_replaces_separators(self):
        """Test replace_path_sep replaces path separators."""
        test_string = f"path{os.path.sep}with{os.path.sep}separators"
        result = replace_path_sep(test_string)
        assert os.path.sep not in result
        assert result == "path_with_separators"

    @pytest.mark.unit
    def test_create_base_filename_structure(self, temp_dir):
        """Test _create_base_filename creates correct structure."""
        title = "test_title"
        output_path = str(temp_dir)
        model = "test_model"
        date = "2024-01-01_12-00-00"
        
        result = _create_base_filename(title, output_path, model, date)
        
        expected_base = f"{date}__{model}__{title}"
        assert expected_base in result
        assert result.startswith(str(temp_dir))

    @pytest.mark.unit
    def test_create_base_filename_with_none_title(self, temp_dir):
        """Test _create_base_filename handles None title."""
        result = _create_base_filename(None, str(temp_dir), "model", "2024-01-01_12-00-00")
        assert "None" in result

    @pytest.mark.unit
    def test_create_base_filename_creates_directory(self, temp_dir):
        """Test create_base_filename creates the directory."""
        title = "test"
        model = "model"
        date = "2024-01-01_12-00-00"
        
        result = create_base_filename(title, str(temp_dir), model, date)
        
        # Directory should be created
        directory = os.path.dirname(result)
        assert os.path.exists(directory)
        assert os.path.isdir(directory)


class TestAudioUtils:
    """Tests for audio utility functions."""

    @pytest.mark.unit
    def test_audio_array_to_sha256(self):
        """Test audio_array_to_sha256 generates consistent hash."""
        audio_array = np.array([0.1, 0.2, 0.3, 0.4, 0.5], dtype=np.float32)
        
        result = audio_array_to_sha256(audio_array)
        
        assert isinstance(result, str)
        assert len(result) == 64  # SHA256 hex digest length
        
        # Same array should produce same hash
        result2 = audio_array_to_sha256(audio_array)
        assert result == result2

    @pytest.mark.unit
    def test_audio_array_to_sha256_different_arrays(self):
        """Test different arrays produce different hashes."""
        array1 = np.array([0.1, 0.2, 0.3], dtype=np.float32)
        array2 = np.array([0.4, 0.5, 0.6], dtype=np.float32)
        
        hash1 = audio_array_to_sha256(array1)
        hash2 = audio_array_to_sha256(array2)
        
        assert hash1 != hash2


class TestPathUtils:
    """Tests for path utility functions."""

    @pytest.mark.unit
    def test_get_path_from_root_single_path(self):
        """Test get_path_from_root with single path."""
        result = get_path_from_root("test_dir")
        
        assert "test_dir" in result
        assert os.path.isabs(result)

    @pytest.mark.unit
    def test_get_path_from_root_multiple_paths(self):
        """Test get_path_from_root with multiple paths."""
        result = get_path_from_root("dir1", "dir2", "file.txt")
        
        assert "dir1" in result
        assert "dir2" in result
        assert "file.txt" in result
        assert os.path.isabs(result)


class TestDictUtils:
    """Tests for dictionary utility functions."""

    @pytest.mark.unit
    def test_get_dict_props(self):
        """Test get_dict_props extracts specified properties."""
        test_dict = {
            "prop1": "value1",
            "prop2": "value2",
            "prop3": "value3",
            "other": "ignored",
        }
        props = ["prop1", "prop2"]
        
        result = get_dict_props(test_dict, props)
        
        assert result == {"prop1": "value1", "prop2": "value2"}
        assert "prop3" not in result
        assert "other" not in result

    @pytest.mark.unit
    def test_get_dict_props_missing_keys(self):
        """Test get_dict_props with missing keys."""
        test_dict = {"prop1": "value1"}
        props = ["prop1", "prop2", "prop3"]
        
        result = get_dict_props(test_dict, props)
        
        assert result == {"prop1": "value1"}
        assert "prop2" not in result


class TestSeedUtils:
    """Tests for seed utility functions."""

    @pytest.mark.unit
    def test_set_seed(self):
        """Test set_seed sets random seeds."""
        seed = 42
        
        # Should not raise any errors
        set_seed(seed)
        
        # Verify numpy seed is set
        import numpy as np
        random_val1 = np.random.rand()
        set_seed(seed)
        random_val2 = np.random.rand()
        assert random_val1 == random_val2

    @pytest.mark.unit
    def test_randomize_seed(self):
        """Test randomize_seed generates random seed."""
        # Test with randomize=True
        seed1 = randomize_seed(42, True)
        seed2 = randomize_seed(42, True)
        
        # Check it returns a numpy uint32
        assert seed1 >= 0
        assert seed2 >= 0
        # Very unlikely to get the same random seed twice when randomizing
        assert seed1 != seed2
        
        # Test with randomize=False (should return the same seed)
        seed3 = randomize_seed(42, False)
        assert seed3 == 42
