"""
Database Models - Data Access Layer

Provides CRUD operations for all database tables.
"""

import json
from datetime import datetime
from typing import Optional, List, Dict, Any
from .connection import execute_query, get_db_cursor


class Generation:
    """Model for TTS generation records."""
    
    @staticmethod
    def create(
        filename: str,
        filepath: str,
        model_name: Optional[str] = None,
        model_type: Optional[str] = None,
        text: Optional[str] = None,
        language: Optional[str] = None,
        voice: Optional[str] = None,
        parameters: Optional[Dict[str, Any]] = None,
        generation_time_seconds: Optional[float] = None,
        file_size: Optional[int] = None,
        duration_seconds: Optional[float] = None,
        user_id: int = 1,
        status: str = "completed",
        error_message: Optional[str] = None
    ) -> int:
        """Create a new generation record."""
        query = """
            INSERT INTO generations 
            (filename, filepath, model_name, model_type, text, language, voice,
             parameters, generation_time_seconds, file_size, duration_seconds,
             user_id, status, error_message)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """
        params = (
            filename, filepath, model_name, model_type, text, language, voice,
            json.dumps(parameters or {}), generation_time_seconds, file_size,
            duration_seconds, user_id, status, error_message
        )
        return execute_query(query, params)
    
    @staticmethod
    def get_by_id(generation_id: int) -> Optional[Dict[str, Any]]:
        """Get a generation by ID."""
        query = "SELECT * FROM generations WHERE id = ?"
        return execute_query(query, (generation_id,), fetch_one=True)
    
    @staticmethod
    def get_by_filepath(filepath: str) -> Optional[Dict[str, Any]]:
        """Get a generation by filepath."""
        query = "SELECT * FROM generations WHERE filepath = ?"
        return execute_query(query, (filepath,), fetch_one=True)
    
    @staticmethod
    def list_all(
        limit: int = 100,
        offset: int = 0,
        model_type: Optional[str] = None,
        model_name: Optional[str] = None,
        user_id: Optional[int] = None,
        status: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """List generations with optional filtering."""
        query = "SELECT * FROM generations WHERE 1=1"
        params = []
        
        if model_type:
            query += " AND model_type = ?"
            params.append(model_type)
        if model_name:
            query += " AND model_name = ?"
            params.append(model_name)
        if user_id:
            query += " AND user_id = ?"
            params.append(user_id)
        if status:
            query += " AND status = ?"
            params.append(status)
        
        query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
        params.extend([limit, offset])
        
        return execute_query(query, tuple(params))
    
    @staticmethod
    def count(
        model_type: Optional[str] = None,
        model_name: Optional[str] = None,
        user_id: Optional[int] = None
    ) -> int:
        """Count generations with optional filtering."""
        query = "SELECT COUNT(*) as count FROM generations WHERE 1=1"
        params = []
        
        if model_type:
            query += " AND model_type = ?"
            params.append(model_type)
        if model_name:
            query += " AND model_name = ?"
            params.append(model_name)
        if user_id:
            query += " AND user_id = ?"
            params.append(user_id)
        
        result = execute_query(query, tuple(params), fetch_one=True)
        return result["count"] if result else 0
    
    @staticmethod
    def update(generation_id: int, **kwargs) -> int:
        """Update a generation record."""
        if not kwargs:
            return 0
        
        # Handle JSON fields
        if "parameters" in kwargs and isinstance(kwargs["parameters"], dict):
            kwargs["parameters"] = json.dumps(kwargs["parameters"])
        
        set_clause = ", ".join(f"{k} = ?" for k in kwargs.keys())
        query = f"UPDATE generations SET {set_clause} WHERE id = ?"
        params = tuple(kwargs.values()) + (generation_id,)
        return execute_query(query, params)
    
    @staticmethod
    def delete(generation_id: int) -> int:
        """Delete a generation record."""
        query = "DELETE FROM generations WHERE id = ?"
        return execute_query(query, (generation_id,))
    
    @staticmethod
    def mark_missing(filepath: str) -> int:
        """Mark a generation as file missing."""
        query = "UPDATE generations SET file_exists = 0 WHERE filepath = ?"
        return execute_query(query, (filepath,))
    
    @staticmethod
    def mark_exists(filepath: str) -> int:
        """Mark a generation file as existing."""
        query = "UPDATE generations SET file_exists = 1 WHERE filepath = ?"
        return execute_query(query, (filepath,))
    
    @staticmethod
    def get_all_filepaths() -> List[str]:
        """Get all filepaths from generations."""
        query = "SELECT filepath FROM generations"
        results = execute_query(query)
        return [r["filepath"] for r in results]


class Favorite:
    """Model for favorites."""
    
    @staticmethod
    def create(
        generation_id: int,
        user_id: int = 1,
        name: Optional[str] = None,
        notes: Optional[str] = None,
        tags: Optional[List[str]] = None
    ) -> int:
        """Add a generation to favorites."""
        query = """
            INSERT INTO favorites (generation_id, user_id, name, notes, tags)
            VALUES (?, ?, ?, ?, ?)
        """
        params = (generation_id, user_id, name, notes, json.dumps(tags or []))
        return execute_query(query, params)
    
    @staticmethod
    def get_by_id(favorite_id: int) -> Optional[Dict[str, Any]]:
        """Get a favorite by ID."""
        query = """
            SELECT f.*, g.* 
            FROM favorites f
            JOIN generations g ON f.generation_id = g.id
            WHERE f.id = ?
        """
        return execute_query(query, (favorite_id,), fetch_one=True)
    
    @staticmethod
    def list_all(user_id: int = 1, limit: int = 100, offset: int = 0) -> List[Dict[str, Any]]:
        """List all favorites for a user."""
        query = """
            SELECT f.id as favorite_id, f.name as favorite_name, f.notes, f.tags, 
                   f.created_at as favorited_at, g.*
            FROM favorites f
            JOIN generations g ON f.generation_id = g.id
            WHERE f.user_id = ?
            ORDER BY f.created_at DESC
            LIMIT ? OFFSET ?
        """
        return execute_query(query, (user_id, limit, offset))
    
    @staticmethod
    def delete(favorite_id: int) -> int:
        """Remove from favorites."""
        query = "DELETE FROM favorites WHERE id = ?"
        return execute_query(query, (favorite_id,))
    
    @staticmethod
    def delete_by_generation(generation_id: int, user_id: int = 1) -> int:
        """Remove a generation from favorites."""
        query = "DELETE FROM favorites WHERE generation_id = ? AND user_id = ?"
        return execute_query(query, (generation_id, user_id))
    
    @staticmethod
    def is_favorited(generation_id: int, user_id: int = 1) -> bool:
        """Check if a generation is favorited."""
        query = "SELECT id FROM favorites WHERE generation_id = ? AND user_id = ?"
        result = execute_query(query, (generation_id, user_id), fetch_one=True)
        return result is not None


class VoiceProfile:
    """Model for voice profiles."""
    
    @staticmethod
    def create(
        name: str,
        model_type: str,
        config: Dict[str, Any],
        description: Optional[str] = None,
        reference_audio_path: Optional[str] = None,
        user_id: int = 1,
        is_default: bool = False
    ) -> int:
        """Create a new voice profile."""
        query = """
            INSERT INTO voice_profiles 
            (name, model_type, config, description, reference_audio_path, user_id, is_default)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        """
        params = (name, model_type, json.dumps(config), description, 
                  reference_audio_path, user_id, is_default)
        return execute_query(query, params)
    
    @staticmethod
    def get_by_id(profile_id: int) -> Optional[Dict[str, Any]]:
        """Get a voice profile by ID."""
        query = "SELECT * FROM voice_profiles WHERE id = ?"
        result = execute_query(query, (profile_id,), fetch_one=True)
        if result and result.get("config"):
            result["config"] = json.loads(result["config"])
        return result
    
    @staticmethod
    def list_all(
        user_id: Optional[int] = None,
        model_type: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """List voice profiles."""
        query = "SELECT * FROM voice_profiles WHERE 1=1"
        params = []
        
        if user_id:
            query += " AND user_id = ?"
            params.append(user_id)
        if model_type:
            query += " AND model_type = ?"
            params.append(model_type)
        
        query += " ORDER BY name"
        results = execute_query(query, tuple(params))
        
        # Parse JSON configs
        for r in results:
            if r.get("config"):
                r["config"] = json.loads(r["config"])
        
        return results
    
    @staticmethod
    def update(profile_id: int, **kwargs) -> int:
        """Update a voice profile."""
        if not kwargs:
            return 0
        
        if "config" in kwargs and isinstance(kwargs["config"], dict):
            kwargs["config"] = json.dumps(kwargs["config"])
        
        kwargs["updated_at"] = datetime.now().isoformat()
        
        set_clause = ", ".join(f"{k} = ?" for k in kwargs.keys())
        query = f"UPDATE voice_profiles SET {set_clause} WHERE id = ?"
        params = tuple(kwargs.values()) + (profile_id,)
        return execute_query(query, params)
    
    @staticmethod
    def delete(profile_id: int) -> int:
        """Delete a voice profile."""
        query = "DELETE FROM voice_profiles WHERE id = ?"
        return execute_query(query, (profile_id,))


class UserPreference:
    """Model for user preferences (key-value store with categories)."""
    
    @staticmethod
    def set(category: str, key: str, value: Any, user_id: int = 1) -> int:
        """Set a preference value."""
        query = """
            INSERT INTO user_preferences (user_id, category, key, value, updated_at)
            VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(user_id, category, key) 
            DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
        """
        return execute_query(query, (user_id, category, key, json.dumps(value)))
    
    @staticmethod
    def get(category: str, key: str, user_id: int = 1, default: Any = None) -> Any:
        """Get a preference value."""
        query = "SELECT value FROM user_preferences WHERE user_id = ? AND category = ? AND key = ?"
        result = execute_query(query, (user_id, category, key), fetch_one=True)
        if result:
            return json.loads(result["value"])
        return default
    
    @staticmethod
    def get_category(category: str, user_id: int = 1) -> Dict[str, Any]:
        """Get all preferences in a category."""
        query = "SELECT key, value FROM user_preferences WHERE user_id = ? AND category = ?"
        results = execute_query(query, (user_id, category))
        return {r["key"]: json.loads(r["value"]) for r in results}
    
    @staticmethod
    def get_all(user_id: int = 1) -> Dict[str, Dict[str, Any]]:
        """Get all preferences grouped by category."""
        query = "SELECT category, key, value FROM user_preferences WHERE user_id = ?"
        results = execute_query(query, (user_id,))
        
        prefs = {}
        for r in results:
            if r["category"] not in prefs:
                prefs[r["category"]] = {}
            prefs[r["category"]][r["key"]] = json.loads(r["value"])
        
        return prefs
    
    @staticmethod
    def delete(category: str, key: str, user_id: int = 1) -> int:
        """Delete a preference."""
        query = "DELETE FROM user_preferences WHERE user_id = ? AND category = ? AND key = ?"
        return execute_query(query, (user_id, category, key))


class User:
    """Model for users (for future authentication)."""
    
    @staticmethod
    def create(
        username: str,
        email: Optional[str] = None,
        password_hash: Optional[str] = None,
        is_admin: bool = False
    ) -> int:
        """Create a new user."""
        query = """
            INSERT INTO users (username, email, password_hash, is_admin)
            VALUES (?, ?, ?, ?)
        """
        return execute_query(query, (username, email, password_hash, is_admin))
    
    @staticmethod
    def get_by_id(user_id: int) -> Optional[Dict[str, Any]]:
        """Get a user by ID."""
        query = "SELECT id, username, email, is_active, is_admin, created_at, settings FROM users WHERE id = ?"
        return execute_query(query, (user_id,), fetch_one=True)
    
    @staticmethod
    def get_by_username(username: str) -> Optional[Dict[str, Any]]:
        """Get a user by username."""
        query = "SELECT * FROM users WHERE username = ?"
        return execute_query(query, (username,), fetch_one=True)
    
    @staticmethod
    def list_all() -> List[Dict[str, Any]]:
        """List all users."""
        query = "SELECT id, username, email, is_active, is_admin, created_at FROM users"
        return execute_query(query)
    
    @staticmethod
    def update(user_id: int, **kwargs) -> int:
        """Update a user."""
        if not kwargs:
            return 0
        
        if "settings" in kwargs and isinstance(kwargs["settings"], dict):
            kwargs["settings"] = json.dumps(kwargs["settings"])
        
        kwargs["updated_at"] = datetime.now().isoformat()
        
        set_clause = ", ".join(f"{k} = ?" for k in kwargs.keys())
        query = f"UPDATE users SET {set_clause} WHERE id = ?"
        params = tuple(kwargs.values()) + (user_id,)
        return execute_query(query, params)


class ApiKey:
    """Model for API keys."""
    
    @staticmethod
    def create(
        key_hash: str,
        key_prefix: str,
        user_id: int = 1,
        name: Optional[str] = None,
        expires_at: Optional[str] = None
    ) -> int:
        """Create a new API key record."""
        query = """
            INSERT INTO api_keys (key_hash, key_prefix, user_id, name, expires_at)
            VALUES (?, ?, ?, ?, ?)
        """
        return execute_query(query, (key_hash, key_prefix, user_id, name, expires_at))
    
    @staticmethod
    def get_by_hash(key_hash: str) -> Optional[Dict[str, Any]]:
        """Get an API key by its hash."""
        query = """
            SELECT ak.*, u.username, u.is_admin 
            FROM api_keys ak
            JOIN users u ON ak.user_id = u.id
            WHERE ak.key_hash = ? AND ak.is_active = 1
        """
        return execute_query(query, (key_hash,), fetch_one=True)
    
    @staticmethod
    def update_last_used(key_id: int) -> int:
        """Update the last used timestamp."""
        query = "UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?"
        return execute_query(query, (key_id,))
    
    @staticmethod
    def list_for_user(user_id: int) -> List[Dict[str, Any]]:
        """List API keys for a user (without the hash)."""
        query = """
            SELECT id, key_prefix, name, is_active, created_at, last_used_at, expires_at
            FROM api_keys WHERE user_id = ?
        """
        return execute_query(query, (user_id,))
    
    @staticmethod
    def revoke(key_id: int) -> int:
        """Revoke an API key."""
        query = "UPDATE api_keys SET is_active = 0 WHERE id = ?"
        return execute_query(query, (key_id,))
