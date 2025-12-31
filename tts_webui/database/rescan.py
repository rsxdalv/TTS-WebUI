"""
Rescan Utility

Scans the outputs directory and synchronizes with the database:
- Adds new files not in database
- Marks missing files
- Extracts metadata where possible
"""

import os
import re
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple


# Audio file extensions to scan
AUDIO_EXTENSIONS = {".wav", ".mp3", ".flac", ".ogg", ".m4a", ".opus"}

# Common output directories
DEFAULT_OUTPUT_DIRS = ["outputs", "favorites", "outputs-rvc"]


def rescan_outputs(
    output_dirs: Optional[List[str]] = None,
    update_missing: bool = True,
    add_new: bool = True
) -> Dict:
    """
    Rescan output directories and sync with database.
    
    Args:
        output_dirs: List of directories to scan. Defaults to common output dirs.
        update_missing: Mark database entries as missing if file doesn't exist
        add_new: Add new files found on disk to database
    
    Returns:
        Dict with scan results:
        - scanned: number of files scanned
        - added: number of new files added
        - marked_missing: number marked as missing
        - already_tracked: number already in database
        - errors: list of errors encountered
    """
    from .models import Generation
    from .connection import init_db
    
    # Ensure database is initialized
    init_db()
    
    if output_dirs is None:
        output_dirs = DEFAULT_OUTPUT_DIRS
    
    results = {
        "scanned": 0,
        "added": 0,
        "marked_missing": 0,
        "already_tracked": 0,
        "errors": [],
        "directories_scanned": []
    }
    
    # Get all tracked filepaths from database
    tracked_paths: Set[str] = set(Generation.get_all_filepaths())
    found_paths: Set[str] = set()
    
    # Scan each directory
    for output_dir in output_dirs:
        if not os.path.exists(output_dir):
            continue
        
        results["directories_scanned"].append(output_dir)
        
        for root, dirs, files in os.walk(output_dir):
            for filename in files:
                ext = os.path.splitext(filename)[1].lower()
                if ext not in AUDIO_EXTENSIONS:
                    continue
                
                filepath = os.path.join(root, filename)
                abs_filepath = os.path.abspath(filepath)
                results["scanned"] += 1
                found_paths.add(abs_filepath)
                
                # Normalize path for comparison
                normalized_path = _normalize_path(filepath)
                
                # Check if already tracked
                if normalized_path in tracked_paths or abs_filepath in tracked_paths:
                    results["already_tracked"] += 1
                    Generation.mark_exists(normalized_path)
                    continue
                
                # Add new file to database
                if add_new:
                    try:
                        metadata = _extract_metadata(filepath, filename)
                        Generation.create(
                            filename=filename,
                            filepath=normalized_path,
                            model_name=metadata.get("model_name"),
                            model_type=metadata.get("model_type", "tts"),
                            text=metadata.get("text"),
                            language=metadata.get("language"),
                            voice=metadata.get("voice"),
                            parameters=metadata.get("parameters", {}),
                            file_size=_get_file_size(filepath),
                            duration_seconds=_get_audio_duration(filepath),
                            status="imported"  # Mark as imported vs generated
                        )
                        results["added"] += 1
                    except Exception as e:
                        results["errors"].append(f"Error adding {filepath}: {e}")
    
    # Mark missing files
    if update_missing:
        for tracked_path in tracked_paths:
            if tracked_path not in found_paths:
                # Check if file really doesn't exist
                if not os.path.exists(tracked_path):
                    Generation.mark_missing(tracked_path)
                    results["marked_missing"] += 1
    
    return results


def _normalize_path(filepath: str) -> str:
    """Normalize a filepath for consistent storage."""
    # Use forward slashes and relative path if under current directory
    abs_path = os.path.abspath(filepath)
    cwd = os.getcwd()
    
    if abs_path.startswith(cwd):
        rel_path = os.path.relpath(abs_path, cwd)
        return rel_path.replace("\\", "/")
    
    return abs_path.replace("\\", "/")


def _get_file_size(filepath: str) -> Optional[int]:
    """Get file size in bytes."""
    try:
        return os.path.getsize(filepath)
    except Exception:
        return None


def _get_audio_duration(filepath: str) -> Optional[float]:
    """Get audio duration in seconds."""
    try:
        from mutagen import File as MutagenFile
        audio = MutagenFile(filepath)
        if audio is not None and hasattr(audio.info, 'length'):
            return audio.info.length
    except ImportError:
        pass
    except Exception:
        pass
    return None


def _extract_metadata(filepath: str, filename: str) -> Dict:
    """
    Extract metadata from filename and file.
    
    Many TTS tools use predictable filename patterns that we can parse.
    """
    metadata = {
        "model_name": None,
        "model_type": "tts",
        "text": None,
        "language": None,
        "voice": None,
        "parameters": {}
    }
    
    # Get parent directory name (often contains model info)
    parent_dir = os.path.basename(os.path.dirname(filepath))
    
    # Common model patterns in directory names
    model_patterns = {
        "bark": "bark",
        "tortoise": "tortoise",
        "kokoro": "kokoro",
        "xtts": "xtts",
        "styletts": "styletts",
        "rvc": "rvc",
        "demucs": "demucs",
        "musicgen": "musicgen",
        "audiocraft": "audiocraft",
        "valle": "valle",
        "f5": "f5",
        "chatterbox": "chatterbox",
        "openvoice": "openvoice",
        "cosyvoice": "cosyvoice",
        "maha": "maha",
        "mms": "mms",
        "seamless": "seamless",
    }
    
    parent_lower = parent_dir.lower()
    for pattern, model in model_patterns.items():
        if pattern in parent_lower:
            metadata["model_name"] = model
            break
    
    # Try to extract info from filename
    name_without_ext = os.path.splitext(filename)[0]
    
    # Common patterns:
    # - "text_voice_timestamp.wav"
    # - "model_text_timestamp.wav"
    # - "timestamp_text.wav"
    
    # Check for timestamp patterns
    timestamp_patterns = [
        r"_(\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2})",  # YYYY-MM-DD_HH-MM-SS
        r"_(\d{14})",  # YYYYMMDDHHMMSS
        r"_(\d{10,13})",  # Unix timestamp
    ]
    
    for pattern in timestamp_patterns:
        match = re.search(pattern, name_without_ext)
        if match:
            # Remove timestamp from name for further parsing
            name_without_ext = name_without_ext[:match.start()]
            break
    
    # If name has underscores, last part might be voice
    parts = name_without_ext.split("_")
    if len(parts) > 1:
        # Often format is: text_voice or voice_text
        # Check if any part looks like a voice name
        voice_indicators = ["voice", "speaker", "v_", "spk"]
        for i, part in enumerate(parts):
            if any(ind in part.lower() for ind in voice_indicators):
                metadata["voice"] = part
                parts.pop(i)
                break
        
        # Remaining parts might be text (joined)
        if parts:
            potential_text = " ".join(parts)
            if len(potential_text) > 3:  # Avoid very short "text"
                metadata["text"] = potential_text[:500]
    
    # Try to read embedded metadata
    try:
        from mutagen import File as MutagenFile
        audio = MutagenFile(filepath)
        if audio is not None:
            # Check for common metadata tags
            for tag in ["title", "TIT2", "TITLE"]:
                if tag in audio:
                    metadata["text"] = str(audio[tag][0]) if isinstance(audio[tag], list) else str(audio[tag])
                    break
            
            for tag in ["artist", "TPE1", "ARTIST"]:
                if tag in audio:
                    metadata["voice"] = str(audio[tag][0]) if isinstance(audio[tag], list) else str(audio[tag])
                    break
    except Exception:
        pass
    
    return metadata


def scan_for_duplicates() -> List[Tuple[int, int, str]]:
    """
    Find duplicate generations (same filepath).
    
    Returns list of (id1, id2, filepath) tuples.
    """
    from .connection import execute_query
    
    query = """
        SELECT g1.id as id1, g2.id as id2, g1.filepath
        FROM generations g1
        JOIN generations g2 ON g1.filepath = g2.filepath AND g1.id < g2.id
    """
    
    return [(r["id1"], r["id2"], r["filepath"]) for r in execute_query(query)]


def cleanup_missing(delete: bool = False) -> Dict:
    """
    Handle generations with missing files.
    
    Args:
        delete: If True, delete records. If False, just report.
    
    Returns:
        Dict with cleanup results
    """
    from .connection import execute_query
    from .models import Generation
    
    missing = execute_query(
        "SELECT id, filepath FROM generations WHERE file_exists = 0"
    )
    
    result = {
        "missing_count": len(missing),
        "deleted": 0,
        "filepaths": [m["filepath"] for m in missing[:100]]  # First 100
    }
    
    if delete:
        for m in missing:
            Generation.delete(m["id"])
            result["deleted"] += 1
    
    return result
