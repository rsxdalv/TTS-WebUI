"""
Unit tests for tts_webui.decorators module.
"""

import os
import pytest
import datetime
from unittest.mock import patch, MagicMock
from pathlib import Path

from tts_webui.decorators.decorator_add_base_filename import (
    format_filename,
    format_date_for_file,
    decorator_add_base_filename,
)
from tts_webui.decorators.decorator_add_date import decorator_add_date
from tts_webui.decorators.decorator_add_model_type import decorator_add_model_type
from tts_webui.decorators.decorator_apply_torch_seed import decorator_apply_torch_seed


class TestFilenameDecorators:
    """Tests for filename decorator functions."""

    @pytest.mark.unit
    def test_format_filename(self):
        """Test format_filename creates correct format."""
        title = "test_title"
        model = "test_model"
        date = datetime.datetime(2024, 1, 1, 12, 0, 0)
        
        result = format_filename(title, model, date)
        
        # The format uses datetime string format with spaces, not underscores
        assert "test_model" in result
        assert "test_title" in result
        assert "2024-01-01" in result

    @pytest.mark.unit
    def test_format_date_for_file(self):
        """Test format_date_for_file formats datetime correctly."""
        date = datetime.datetime(2024, 1, 15, 14, 30, 45)
        
        result = format_date_for_file(date)
        
        assert result == "2024-01-15_14-30-45"

    @pytest.mark.unit
    def test_decorator_add_base_filename(self, temp_dir):
        """Test decorator_add_base_filename adds filename to result."""
        @decorator_add_base_filename
        def test_function(**kwargs):
            return {
                "audio_array": "test_audio",
                "_type": kwargs.get("_type", "default"),
                "date": datetime.datetime.now()  # Required by decorator
            }
        
        # Test the decorator works
        result = test_function(
            title="test",
            output_path=str(temp_dir),
            model="test_model",
            _type="test_type",  # Required by decorator
        )
        
        assert "filename" in result
        assert "audio_array" in result
        assert isinstance(result["filename"], str)

    @pytest.mark.unit
    def test_decorator_add_base_filename_preserves_existing_keys(self, temp_dir):
        """Test decorator preserves existing result keys."""
        @decorator_add_base_filename
        def test_function(**kwargs):
            return {
                "audio_array": "test_audio",
                "metadata": {"key": "value"},
                "_type": kwargs.get("_type", "default"),
                "date": datetime.datetime.now()  # Required by decorator
            }
        
        result = test_function(
            title="test",
            output_path=str(temp_dir),
            model="test_model",
            _type="test_type",  # Required by decorator
        )
        
        assert result["audio_array"] == "test_audio"
        assert result["metadata"]["key"] == "value"
        assert "filename" in result


class TestDateDecorators:
    """Tests for date decorator functions."""

    @pytest.mark.unit
    def test_decorator_add_date(self):
        """Test decorator_add_date adds date to result."""
        @decorator_add_date
        def test_function(**kwargs):
            return {"audio_array": "test"}
        
        result = test_function()
        
        assert "date" in result
        assert isinstance(result["date"], datetime.datetime)
        assert "audio_array" in result

    @pytest.mark.unit
    def test_decorator_add_date_generator(self):
        """Test decorator_add_date_generator works with generators."""
        from tts_webui.decorators.decorator_add_date import decorator_add_date_generator
        
        @decorator_add_date_generator
        def test_generator(**kwargs):
            yield {"audio_array": "test1"}
            yield {"audio_array": "test2"}
        
        results = list(test_generator())
        
        assert len(results) == 2
        for result in results:
            assert "date" in result
            assert isinstance(result["date"], datetime.datetime)


class TestModelTypeDecorators:
    """Tests for model type decorator functions."""

    @pytest.mark.unit
    def test_decorator_add_model_type(self):
        """Test decorator_add_model_type adds _type parameter."""
        @decorator_add_model_type("test_model_type")
        def test_function(**kwargs):
            # Decorator adds _type to kwargs, function can use it
            return {"audio_array": "test", "_type": kwargs.get("_type")}
        
        result = test_function()
        
        assert "_type" in result
        assert result["_type"] == "test_model_type"
        assert "audio_array" in result

    @pytest.mark.unit
    def test_decorator_add_model_type_generator(self):
        """Test decorator_add_model_type_generator works with generators."""
        from tts_webui.decorators.decorator_add_model_type import decorator_add_model_type_generator
        
        @decorator_add_model_type_generator("test_model")
        def test_generator(**kwargs):
            yield {"audio_array": "test1", "_type": kwargs.get("_type")}
            yield {"audio_array": "test2", "_type": kwargs.get("_type")}
        
        results = list(test_generator())
        
        assert len(results) == 2
        for result in results:
            assert "_type" in result
            assert result["_type"] == "test_model"


class TestTorchSeedDecorators:
    """Tests for torch seed decorator functions."""

    @pytest.mark.unit
    def test_decorator_apply_torch_seed(self):
        """Test decorator_apply_torch_seed applies seed."""
        @decorator_apply_torch_seed
        def test_function(**kwargs):
            # Simulate function that benefits from seeding
            return {"result": "test", "seed_used": kwargs.get("seed")}
        
        result1 = test_function(seed=42)
        result2 = test_function(seed=42)
        
        assert result1["seed_used"] == 42
        assert result2["seed_used"] == 42

    @pytest.mark.unit
    def test_decorator_apply_torch_seed_without_seed(self):
        """Test decorator works when no seed is provided."""
        @decorator_apply_torch_seed
        def test_function(**kwargs):
            return {"result": "test"}
        
        # Should not raise an error
        result = test_function()
        assert "result" in result


class TestDecoratorChaining:
    """Tests for chaining multiple decorators."""

    @pytest.mark.unit
    def test_chained_decorators(self, temp_dir):
        """Test that multiple decorators can be chained."""
        @decorator_add_date
        @decorator_add_model_type("chained_model")
        def test_function(**kwargs):
            return {"audio_array": "test", "_type": kwargs.get("_type")}
        
        result = test_function()
        
        assert "date" in result
        assert "_type" in result
        assert result["_type"] == "chained_model"
        assert "audio_array" in result
