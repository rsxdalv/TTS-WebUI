"""
REST API Server for TTS WebUI Database

A standalone REST API server that provides database access for React UI.
Runs separately from the Gradio server on a different port.
"""

import os
import hashlib
import secrets
from functools import wraps
from typing import Optional
from http import HTTPStatus

from flask import Flask, request, jsonify, g
from flask_cors import CORS

from .connection import init_db, get_db_path
from .models import Generation, Favorite, VoiceProfile, UserPreference, User, ApiKey

# Create Flask app
app = Flask(__name__)
CORS(app, origins=["http://localhost:3000", "http://127.0.0.1:3000"])

# Configuration
API_PORT = int(os.environ.get("TTS_WEBUI_API_PORT", 7774))
API_HOST = os.environ.get("TTS_WEBUI_API_HOST", "127.0.0.1")


# ============================================================================
# Authentication
# ============================================================================

def hash_api_key(key: str) -> str:
    """Hash an API key for storage."""
    return hashlib.sha256(key.encode()).hexdigest()


def generate_api_key() -> tuple[str, str, str]:
    """Generate a new API key. Returns (full_key, prefix, hash)."""
    prefix = "tts_" + secrets.token_hex(4)
    secret = secrets.token_hex(24)
    full_key = f"{prefix}_{secret}"
    key_hash = hash_api_key(full_key)
    return full_key, prefix, key_hash


def require_auth(f):
    """Decorator to require API key authentication."""
    @wraps(f)
    def decorated(*args, **kwargs):
        # Check for API key in header
        auth_header = request.headers.get("Authorization")
        api_key = request.headers.get("X-API-Key")
        
        key = None
        if auth_header and auth_header.startswith("Bearer "):
            key = auth_header[7:]
        elif api_key:
            key = api_key
        
        if not key:
            return jsonify({"error": "API key required"}), HTTPStatus.UNAUTHORIZED
        
        # Validate the key
        key_hash = hash_api_key(key)
        key_record = ApiKey.get_by_hash(key_hash)
        
        if not key_record:
            return jsonify({"error": "Invalid API key"}), HTTPStatus.UNAUTHORIZED
        
        # Check expiration
        if key_record.get("expires_at"):
            from datetime import datetime
            expires = datetime.fromisoformat(key_record["expires_at"])
            if datetime.now() > expires:
                return jsonify({"error": "API key expired"}), HTTPStatus.UNAUTHORIZED
        
        # Update last used
        ApiKey.update_last_used(key_record["id"])
        
        # Store user info in request context
        g.user_id = key_record["user_id"]
        g.is_admin = key_record["is_admin"]
        
        return f(*args, **kwargs)
    return decorated


def optional_auth(f):
    """Decorator for optional authentication - uses default user if no key provided."""
    @wraps(f)
    def decorated(*args, **kwargs):
        auth_header = request.headers.get("Authorization")
        api_key = request.headers.get("X-API-Key")
        
        key = None
        if auth_header and auth_header.startswith("Bearer "):
            key = auth_header[7:]
        elif api_key:
            key = api_key
        
        if key:
            key_hash = hash_api_key(key)
            key_record = ApiKey.get_by_hash(key_hash)
            if key_record:
                ApiKey.update_last_used(key_record["id"])
                g.user_id = key_record["user_id"]
                g.is_admin = key_record["is_admin"]
            else:
                g.user_id = 1
                g.is_admin = True
        else:
            # Default user for unauthenticated requests
            g.user_id = 1
            g.is_admin = True
        
        return f(*args, **kwargs)
    return decorated


# ============================================================================
# Health & Status
# ============================================================================

@app.route("/api/health", methods=["GET"])
def health():
    """Health check endpoint."""
    return jsonify({
        "status": "ok",
        "database": str(get_db_path()),
        "version": "1.0.0"
    })


# ============================================================================
# API Key Management
# ============================================================================

@app.route("/api/keys", methods=["POST"])
@optional_auth
def create_api_key():
    """Create a new API key."""
    data = request.get_json() or {}
    name = data.get("name", "Unnamed Key")
    expires_at = data.get("expires_at")
    
    full_key, prefix, key_hash = generate_api_key()
    
    ApiKey.create(
        key_hash=key_hash,
        key_prefix=prefix,
        user_id=g.user_id,
        name=name,
        expires_at=expires_at
    )
    
    return jsonify({
        "key": full_key,
        "prefix": prefix,
        "name": name,
        "message": "Save this key - it won't be shown again!"
    }), HTTPStatus.CREATED


@app.route("/api/keys", methods=["GET"])
@optional_auth
def list_api_keys():
    """List API keys for the current user."""
    keys = ApiKey.list_for_user(g.user_id)
    return jsonify({"keys": keys})


@app.route("/api/keys/<int:key_id>", methods=["DELETE"])
@optional_auth
def revoke_api_key(key_id: int):
    """Revoke an API key."""
    ApiKey.revoke(key_id)
    return jsonify({"message": "Key revoked"})


# ============================================================================
# Generations API
# ============================================================================

@app.route("/api/generations", methods=["GET"])
@optional_auth
def list_generations():
    """List generation history."""
    limit = request.args.get("limit", 100, type=int)
    offset = request.args.get("offset", 0, type=int)
    model_type = request.args.get("model_type")
    model_name = request.args.get("model_name")
    status = request.args.get("status")
    
    generations = Generation.list_all(
        limit=min(limit, 500),
        offset=offset,
        model_type=model_type,
        model_name=model_name,
        status=status
    )
    
    total = Generation.count(
        model_type=model_type,
        model_name=model_name
    )
    
    return jsonify({
        "generations": generations,
        "total": total,
        "limit": limit,
        "offset": offset
    })


@app.route("/api/generations/<int:generation_id>", methods=["GET"])
@optional_auth
def get_generation(generation_id: int):
    """Get a specific generation."""
    generation = Generation.get_by_id(generation_id)
    if not generation:
        return jsonify({"error": "Generation not found"}), HTTPStatus.NOT_FOUND
    return jsonify(generation)


@app.route("/api/generations", methods=["POST"])
@optional_auth
def create_generation():
    """Create a new generation record."""
    data = request.get_json()
    if not data:
        return jsonify({"error": "Request body required"}), HTTPStatus.BAD_REQUEST
    
    required = ["filename", "filepath"]
    for field in required:
        if field not in data:
            return jsonify({"error": f"Missing required field: {field}"}), HTTPStatus.BAD_REQUEST
    
    generation_id = Generation.create(
        filename=data["filename"],
        filepath=data["filepath"],
        model_name=data.get("model_name"),
        model_type=data.get("model_type"),
        text=data.get("text"),
        language=data.get("language"),
        voice=data.get("voice"),
        parameters=data.get("parameters"),
        generation_time_seconds=data.get("generation_time_seconds"),
        file_size=data.get("file_size"),
        duration_seconds=data.get("duration_seconds"),
        user_id=g.user_id,
        status=data.get("status", "completed"),
        error_message=data.get("error_message")
    )
    
    return jsonify({"id": generation_id}), HTTPStatus.CREATED


@app.route("/api/generations/<int:generation_id>", methods=["PATCH"])
@optional_auth
def update_generation(generation_id: int):
    """Update a generation record."""
    data = request.get_json()
    if not data:
        return jsonify({"error": "Request body required"}), HTTPStatus.BAD_REQUEST
    
    # Only allow updating certain fields
    allowed_fields = ["file_exists", "status", "error_message", "parameters"]
    updates = {k: v for k, v in data.items() if k in allowed_fields}
    
    if updates:
        Generation.update(generation_id, **updates)
    
    return jsonify({"message": "Updated"})


@app.route("/api/generations/<int:generation_id>", methods=["DELETE"])
@optional_auth
def delete_generation(generation_id: int):
    """Delete a generation record."""
    Generation.delete(generation_id)
    return jsonify({"message": "Deleted"})


# ============================================================================
# Favorites API
# ============================================================================

@app.route("/api/favorites", methods=["GET"])
@optional_auth
def list_favorites():
    """List all favorites."""
    limit = request.args.get("limit", 100, type=int)
    offset = request.args.get("offset", 0, type=int)
    
    favorites = Favorite.list_all(
        user_id=g.user_id,
        limit=min(limit, 500),
        offset=offset
    )
    
    return jsonify({"favorites": favorites})


@app.route("/api/favorites", methods=["POST"])
@optional_auth
def add_favorite():
    """Add a generation to favorites."""
    data = request.get_json()
    if not data or "generation_id" not in data:
        return jsonify({"error": "generation_id required"}), HTTPStatus.BAD_REQUEST
    
    try:
        favorite_id = Favorite.create(
            generation_id=data["generation_id"],
            user_id=g.user_id,
            name=data.get("name"),
            notes=data.get("notes"),
            tags=data.get("tags")
        )
        return jsonify({"id": favorite_id}), HTTPStatus.CREATED
    except Exception as e:
        if "UNIQUE constraint" in str(e):
            return jsonify({"error": "Already favorited"}), HTTPStatus.CONFLICT
        raise


@app.route("/api/favorites/<int:favorite_id>", methods=["DELETE"])
@optional_auth
def remove_favorite(favorite_id: int):
    """Remove from favorites."""
    Favorite.delete(favorite_id)
    return jsonify({"message": "Removed from favorites"})


@app.route("/api/favorites/generation/<int:generation_id>", methods=["DELETE"])
@optional_auth
def unfavorite_generation(generation_id: int):
    """Remove a generation from favorites."""
    Favorite.delete_by_generation(generation_id, g.user_id)
    return jsonify({"message": "Removed from favorites"})


@app.route("/api/favorites/check/<int:generation_id>", methods=["GET"])
@optional_auth
def check_favorite(generation_id: int):
    """Check if a generation is favorited."""
    is_fav = Favorite.is_favorited(generation_id, g.user_id)
    return jsonify({"is_favorited": is_fav})


# ============================================================================
# Voice Profiles API
# ============================================================================

@app.route("/api/voice-profiles", methods=["GET"])
@optional_auth
def list_voice_profiles():
    """List voice profiles."""
    model_type = request.args.get("model_type")
    
    profiles = VoiceProfile.list_all(
        user_id=g.user_id,
        model_type=model_type
    )
    
    return jsonify({"profiles": profiles})


@app.route("/api/voice-profiles/<int:profile_id>", methods=["GET"])
@optional_auth
def get_voice_profile(profile_id: int):
    """Get a voice profile."""
    profile = VoiceProfile.get_by_id(profile_id)
    if not profile:
        return jsonify({"error": "Profile not found"}), HTTPStatus.NOT_FOUND
    return jsonify(profile)


@app.route("/api/voice-profiles", methods=["POST"])
@optional_auth
def create_voice_profile():
    """Create a voice profile."""
    data = request.get_json()
    if not data:
        return jsonify({"error": "Request body required"}), HTTPStatus.BAD_REQUEST
    
    required = ["name", "model_type", "config"]
    for field in required:
        if field not in data:
            return jsonify({"error": f"Missing required field: {field}"}), HTTPStatus.BAD_REQUEST
    
    profile_id = VoiceProfile.create(
        name=data["name"],
        model_type=data["model_type"],
        config=data["config"],
        description=data.get("description"),
        reference_audio_path=data.get("reference_audio_path"),
        user_id=g.user_id,
        is_default=data.get("is_default", False)
    )
    
    return jsonify({"id": profile_id}), HTTPStatus.CREATED


@app.route("/api/voice-profiles/<int:profile_id>", methods=["PATCH"])
@optional_auth
def update_voice_profile(profile_id: int):
    """Update a voice profile."""
    data = request.get_json()
    if not data:
        return jsonify({"error": "Request body required"}), HTTPStatus.BAD_REQUEST
    
    allowed_fields = ["name", "description", "config", "reference_audio_path", "is_default"]
    updates = {k: v for k, v in data.items() if k in allowed_fields}
    
    if updates:
        VoiceProfile.update(profile_id, **updates)
    
    return jsonify({"message": "Updated"})


@app.route("/api/voice-profiles/<int:profile_id>", methods=["DELETE"])
@optional_auth
def delete_voice_profile(profile_id: int):
    """Delete a voice profile."""
    VoiceProfile.delete(profile_id)
    return jsonify({"message": "Deleted"})


# ============================================================================
# User Preferences API
# ============================================================================

@app.route("/api/preferences", methods=["GET"])
@optional_auth
def get_all_preferences():
    """Get all preferences."""
    category = request.args.get("category")
    
    if category:
        prefs = UserPreference.get_category(category, g.user_id)
    else:
        prefs = UserPreference.get_all(g.user_id)
    
    return jsonify({"preferences": prefs})


@app.route("/api/preferences/<category>/<key>", methods=["GET"])
@optional_auth
def get_preference(category: str, key: str):
    """Get a specific preference."""
    value = UserPreference.get(category, key, g.user_id)
    return jsonify({"value": value})


@app.route("/api/preferences/<category>/<key>", methods=["PUT"])
@optional_auth
def set_preference(category: str, key: str):
    """Set a preference value."""
    data = request.get_json()
    if data is None or "value" not in data:
        return jsonify({"error": "value required"}), HTTPStatus.BAD_REQUEST
    
    UserPreference.set(category, key, data["value"], g.user_id)
    return jsonify({"message": "Preference saved"})


@app.route("/api/preferences/<category>/<key>", methods=["DELETE"])
@optional_auth
def delete_preference(category: str, key: str):
    """Delete a preference."""
    UserPreference.delete(category, key, g.user_id)
    return jsonify({"message": "Preference deleted"})


@app.route("/api/preferences/bulk", methods=["PUT"])
@optional_auth
def set_bulk_preferences():
    """Set multiple preferences at once."""
    data = request.get_json()
    if not data or "preferences" not in data:
        return jsonify({"error": "preferences object required"}), HTTPStatus.BAD_REQUEST
    
    # Expected format: { "preferences": { "category": { "key": value, ... }, ... } }
    for category, prefs in data["preferences"].items():
        for key, value in prefs.items():
            UserPreference.set(category, key, value, g.user_id)
    
    return jsonify({"message": "Preferences saved"})


# ============================================================================
# Rescan API
# ============================================================================

@app.route("/api/rescan", methods=["POST"])
@optional_auth
def rescan_outputs():
    """Rescan the outputs directory and sync with database."""
    from .rescan import rescan_outputs as do_rescan
    
    result = do_rescan()
    return jsonify(result)


# ============================================================================
# Statistics API
# ============================================================================

@app.route("/api/stats", methods=["GET"])
@optional_auth
def get_stats():
    """Get database statistics."""
    from .connection import execute_query
    
    stats = {
        "generations": {
            "total": Generation.count(),
            "by_model": execute_query(
                "SELECT model_type, COUNT(*) as count FROM generations GROUP BY model_type"
            )
        },
        "favorites": {
            "total": execute_query(
                "SELECT COUNT(*) as count FROM favorites WHERE user_id = ?",
                (g.user_id,),
                fetch_one=True
            )["count"]
        },
        "voice_profiles": {
            "total": execute_query(
                "SELECT COUNT(*) as count FROM voice_profiles WHERE user_id = ?",
                (g.user_id,),
                fetch_one=True
            )["count"]
        }
    }
    
    return jsonify(stats)


# ============================================================================
# Error Handlers
# ============================================================================

@app.errorhandler(HTTPStatus.NOT_FOUND)
def not_found(e):
    return jsonify({"error": "Not found"}), HTTPStatus.NOT_FOUND


@app.errorhandler(HTTPStatus.INTERNAL_SERVER_ERROR)
def internal_error(e):
    return jsonify({"error": "Internal server error"}), HTTPStatus.INTERNAL_SERVER_ERROR


@app.errorhandler(Exception)
def handle_exception(e):
    # Log the error
    print(f"API Error: {e}")
    return jsonify({"error": str(e)}), HTTPStatus.INTERNAL_SERVER_ERROR


# ============================================================================
# Server Start
# ============================================================================

def start_api_server(host: Optional[str] = None, port: Optional[int] = None, debug: bool = False):
    """Start the REST API server."""
    # Initialize database
    init_db()
    
    h = host or API_HOST
    p = port or API_PORT
    
    print(f"\nStarting TTS WebUI REST API server...")
    print(f"  • URL: http://{h}:{p}")
    print(f"  • Database: {get_db_path()}")
    print(f"  • Docs: http://{h}:{p}/api/health")
    
    # Use use_reloader=False to prevent issues when running as a module
    app.run(host=h, port=p, debug=debug, threaded=True, use_reloader=False)


if __name__ == "__main__":
    import sys
    debug_mode = "--debug" in sys.argv
    start_api_server(debug=debug_mode)

