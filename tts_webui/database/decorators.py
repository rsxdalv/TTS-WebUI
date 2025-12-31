"""
Generation Logging Decorator

A decorator that logs TTS generations to the database without failing on errors.
Can be applied to any generation function.
"""

import os
import time
import traceback
from functools import wraps
from pathlib import Path
from typing import Any, Callable, Dict, Optional, Union


def log_generation(
    model_name: Optional[str] = None,
    model_type: Optional[str] = None,
    extract_text: Optional[Callable[[Any], str]] = None,
    extract_filepath: Optional[Callable[[Any], str]] = None,
    extract_params: Optional[Callable[[Any], Dict]] = None
):
    """
    Decorator to log TTS generations to the database.
    
    Designed to be non-blocking and fail-safe - any database errors
    are logged but don't affect the generation result.
    
    Args:
        model_name: Name of the model (e.g., "kokoro", "bark")
        model_type: Type of model (e.g., "tts", "voice_conversion", "music")
        extract_text: Function to extract text from generation args/kwargs
        extract_filepath: Function to extract output filepath from result
        extract_params: Function to extract parameters from args/kwargs
    
    Example:
        @log_generation(model_name="bark", model_type="tts")
        def generate_bark(text, voice, **kwargs):
            # ... generation code ...
            return output_path
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs):
            start_time = time.time()
            result = None
            error_message = None
            status = "completed"
            
            try:
                # Run the actual generation
                result = func(*args, **kwargs)
                return result
            
            except Exception as e:
                error_message = str(e)
                status = "failed"
                raise
            
            finally:
                # Log to database (fail-safe)
                try:
                    _log_to_database(
                        result=result,
                        args=args,
                        kwargs=kwargs,
                        model_name=model_name,
                        model_type=model_type,
                        extract_text=extract_text,
                        extract_filepath=extract_filepath,
                        extract_params=extract_params,
                        generation_time=time.time() - start_time,
                        status=status,
                        error_message=error_message
                    )
                except Exception as log_error:
                    # Never let logging errors affect the generation
                    print(f"[Database] Warning: Failed to log generation: {log_error}")
        
        return wrapper
    return decorator


def _log_to_database(
    result: Any,
    args: tuple,
    kwargs: dict,
    model_name: Optional[str],
    model_type: Optional[str],
    extract_text: Optional[Callable],
    extract_filepath: Optional[Callable],
    extract_params: Optional[Callable],
    generation_time: float,
    status: str,
    error_message: Optional[str]
):
    """Internal function to log generation to database."""
    from .models import Generation
    from .connection import init_db
    
    # Ensure database is initialized
    try:
        init_db()
    except Exception:
        pass
    
    # Extract filepath
    filepath = None
    if result is not None:
        if extract_filepath:
            filepath = extract_filepath(result)
        elif isinstance(result, (str, Path)):
            filepath = str(result)
        elif isinstance(result, dict) and "path" in result:
            filepath = result["path"]
        elif isinstance(result, tuple) and len(result) > 0:
            # Often the first element is the path
            first = result[0]
            if isinstance(first, (str, Path)) and (
                str(first).endswith(('.wav', '.mp3', '.flac', '.ogg'))
            ):
                filepath = str(first)
    
    if not filepath and status != "failed":
        # Can't log without a filepath
        return
    
    if not filepath:
        filepath = f"error_{int(time.time())}"
    
    # Extract filename
    filename = os.path.basename(filepath) if filepath else "unknown"
    
    # Extract text
    text = None
    if extract_text:
        text = extract_text(args, kwargs)
    elif len(args) > 0 and isinstance(args[0], str):
        text = args[0]
    elif "text" in kwargs:
        text = kwargs["text"]
    elif "prompt" in kwargs:
        text = kwargs["prompt"]
    
    # Extract parameters
    parameters = {}
    if extract_params:
        parameters = extract_params(args, kwargs)
    else:
        # Default: capture all kwargs except very large ones
        for k, v in kwargs.items():
            if k in ("text", "prompt"):
                continue
            if isinstance(v, (str, int, float, bool, list, dict)):
                # Skip very large values
                if isinstance(v, str) and len(v) > 1000:
                    continue
                if isinstance(v, (list, dict)) and len(str(v)) > 1000:
                    continue
                parameters[k] = v
    
    # Get file size if exists
    file_size = None
    if filepath and os.path.exists(filepath):
        try:
            file_size = os.path.getsize(filepath)
        except Exception:
            pass
    
    # Get audio duration (if mutagen is available)
    duration_seconds = None
    if filepath and os.path.exists(filepath):
        try:
            from mutagen import File as MutagenFile
            audio = MutagenFile(filepath)
            if audio is not None and hasattr(audio.info, 'length'):
                duration_seconds = audio.info.length
        except ImportError:
            pass
        except Exception:
            pass
    
    # Extract voice
    voice = kwargs.get("voice") or kwargs.get("speaker") or kwargs.get("voice_name")
    
    # Extract language
    language = kwargs.get("language") or kwargs.get("lang")
    
    # Create database record
    Generation.create(
        filename=filename,
        filepath=str(filepath),
        model_name=model_name or kwargs.get("model_name", "unknown"),
        model_type=model_type or "tts",
        text=text[:5000] if text else None,  # Limit text length
        language=language,
        voice=voice,
        parameters=parameters,
        generation_time_seconds=generation_time,
        file_size=file_size,
        duration_seconds=duration_seconds,
        status=status,
        error_message=error_message
    )


def log_generation_manual(
    filepath: str,
    model_name: str,
    model_type: str = "tts",
    text: Optional[str] = None,
    language: Optional[str] = None,
    voice: Optional[str] = None,
    parameters: Optional[Dict] = None,
    generation_time_seconds: Optional[float] = None,
    status: str = "completed",
    error_message: Optional[str] = None
) -> Optional[int]:
    """
    Manually log a generation to the database.
    
    Use this when the decorator approach doesn't fit.
    Returns the generation ID or None if logging failed.
    """
    try:
        from .models import Generation
        from .connection import init_db
        
        init_db()
        
        filename = os.path.basename(filepath)
        
        file_size = None
        if os.path.exists(filepath):
            try:
                file_size = os.path.getsize(filepath)
            except Exception:
                pass
        
        return Generation.create(
            filename=filename,
            filepath=filepath,
            model_name=model_name,
            model_type=model_type,
            text=text[:5000] if text else None,
            language=language,
            voice=voice,
            parameters=parameters or {},
            generation_time_seconds=generation_time_seconds,
            file_size=file_size,
            status=status,
            error_message=error_message
        )
    
    except Exception as e:
        print(f"[Database] Warning: Failed to log generation: {e}")
        return None
