"""
REST API Server for TTS WebUI Database

A standalone REST API server that provides database access for React UI.
Uses FastAPI/Uvicorn for consistency with OpenAI TTS API.
Runs separately from the Gradio server on a different port.
"""

import os
import hashlib
import secrets
import logging
from typing import Optional, List, Dict, Any
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, Request, HTTPException, Depends, Header
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field

from .connection import init_db, get_db_path
from .models import Generation, Favorite, VoiceProfile, UserPreference, User, ApiKey

logger = logging.getLogger(__name__)

# Configuration
API_PORT = int(os.environ.get("TTS_WEBUI_API_PORT", 7774))
API_HOST = os.environ.get("TTS_WEBUI_API_HOST", "127.0.0.1")


# ============================================================================
# Pydantic Models
# ============================================================================

class HealthResponse(BaseModel):
    status: str
    database: str
    version: str


class CreateApiKeyRequest(BaseModel):
    name: str = "Unnamed Key"
    expires_at: Optional[str] = None


class CreateApiKeyResponse(BaseModel):
    key: str
    prefix: str
    name: str
    message: str


class ApiKeyInfo(BaseModel):
    id: int
    key_prefix: str
    name: Optional[str]
    is_active: bool
    created_at: str
    last_used_at: Optional[str]
    expires_at: Optional[str]


class GenerationCreate(BaseModel):
    filename: str
    filepath: str
    model_name: Optional[str] = None
    model_type: Optional[str] = None
    text: Optional[str] = None
    language: Optional[str] = None
    voice: Optional[str] = None
    parameters: Optional[Dict[str, Any]] = None
    generation_time_seconds: Optional[float] = None
    file_size: Optional[int] = None
    duration_seconds: Optional[float] = None
    status: str = "completed"
    error_message: Optional[str] = None


class GenerationUpdate(BaseModel):
    file_exists: Optional[bool] = None
    status: Optional[str] = None
    error_message: Optional[str] = None
    parameters: Optional[Dict[str, Any]] = None


class FavoriteCreate(BaseModel):
    generation_id: int
    name: Optional[str] = None
    notes: Optional[str] = None
    tags: Optional[List[str]] = None


class VoiceProfileCreate(BaseModel):
    name: str
    model_type: str
    config: Dict[str, Any]
    description: Optional[str] = None
    reference_audio_path: Optional[str] = None
    is_default: bool = False


class VoiceProfileUpdate(BaseModel):
    name: Optional[str] = None
    description: Optional[str] = None
    config: Optional[Dict[str, Any]] = None
    reference_audio_path: Optional[str] = None
    is_default: Optional[bool] = None


class PreferenceValue(BaseModel):
    value: Any


class BulkPreferences(BaseModel):
    preferences: Dict[str, Dict[str, Any]]


class MessageResponse(BaseModel):
    message: str


class IdResponse(BaseModel):
    id: int


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


class AuthContext:
    """Authentication context for the current request."""
    def __init__(self, user_id: int = 1, is_admin: bool = True):
        self.user_id = user_id
        self.is_admin = is_admin


async def get_auth(
    authorization: Optional[str] = Header(None),
    x_api_key: Optional[str] = Header(None, alias="X-API-Key")
) -> AuthContext:
    """
    Dependency for optional authentication.
    Uses default user if no key provided.
    """
    key = None
    if authorization and authorization.startswith("Bearer "):
        key = authorization[7:]
    elif x_api_key:
        key = x_api_key
    
    if key:
        key_hash = hash_api_key(key)
        key_record = ApiKey.get_by_hash(key_hash)
        if key_record:
            # Check expiration
            if key_record.get("expires_at"):
                from datetime import datetime
                expires = datetime.fromisoformat(key_record["expires_at"])
                if datetime.now() > expires:
                    raise HTTPException(status_code=401, detail="API key expired")
            
            ApiKey.update_last_used(key_record["id"])
            return AuthContext(
                user_id=key_record["user_id"],
                is_admin=key_record["is_admin"]
            )
    
    # Default user for unauthenticated requests
    return AuthContext(user_id=1, is_admin=True)


# ============================================================================
# FastAPI App
# ============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize database on startup."""
    init_db()
    logger.info(f"Database initialized at: {get_db_path()}")
    yield


app = FastAPI(
    title="TTS WebUI Database API",
    description="REST API for TTS WebUI database operations",
    version="1.0.0",
    lifespan=lifespan
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*", "http://localhost:3000", "http://127.0.0.1:3000"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ============================================================================
# Health & Status
# ============================================================================

@app.get("/api/health", response_model=HealthResponse)
async def health():
    """Health check endpoint."""
    return HealthResponse(
        status="ok",
        database=str(get_db_path()),
        version="1.0.0"
    )


# ============================================================================
# API Key Management
# ============================================================================

@app.post("/api/keys", response_model=CreateApiKeyResponse, status_code=201)
async def create_api_key(
    data: CreateApiKeyRequest,
    auth: AuthContext = Depends(get_auth)
):
    """Create a new API key."""
    full_key, prefix, key_hash = generate_api_key()
    
    ApiKey.create(
        key_hash=key_hash,
        key_prefix=prefix,
        user_id=auth.user_id,
        name=data.name,
        expires_at=data.expires_at
    )
    
    return CreateApiKeyResponse(
        key=full_key,
        prefix=prefix,
        name=data.name,
        message="Save this key - it won't be shown again!"
    )


@app.get("/api/keys")
async def list_api_keys(auth: AuthContext = Depends(get_auth)):
    """List API keys for the current user."""
    keys = ApiKey.list_for_user(auth.user_id)
    return {"keys": keys}


@app.delete("/api/keys/{key_id}", response_model=MessageResponse)
async def revoke_api_key(key_id: int, auth: AuthContext = Depends(get_auth)):
    """Revoke an API key."""
    ApiKey.revoke(key_id)
    return MessageResponse(message="Key revoked")


# ============================================================================
# Generations API
# ============================================================================

@app.get("/api/generations")
async def list_generations(
    limit: int = 100,
    offset: int = 0,
    model_type: Optional[str] = None,
    model_name: Optional[str] = None,
    status: Optional[str] = None,
    auth: AuthContext = Depends(get_auth)
):
    """List generation history."""
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
    
    return {
        "generations": generations,
        "total": total,
        "limit": limit,
        "offset": offset
    }


@app.get("/api/generations/{generation_id}")
async def get_generation(generation_id: int, auth: AuthContext = Depends(get_auth)):
    """Get a specific generation."""
    generation = Generation.get_by_id(generation_id)
    if not generation:
        raise HTTPException(status_code=404, detail="Generation not found")
    return generation


@app.post("/api/generations", response_model=IdResponse, status_code=201)
async def create_generation(
    data: GenerationCreate,
    auth: AuthContext = Depends(get_auth)
):
    """Create a new generation record."""
    generation_id = Generation.create(
        filename=data.filename,
        filepath=data.filepath,
        model_name=data.model_name,
        model_type=data.model_type,
        text=data.text,
        language=data.language,
        voice=data.voice,
        parameters=data.parameters,
        generation_time_seconds=data.generation_time_seconds,
        file_size=data.file_size,
        duration_seconds=data.duration_seconds,
        user_id=auth.user_id,
        status=data.status,
        error_message=data.error_message
    )
    
    return IdResponse(id=generation_id)


@app.patch("/api/generations/{generation_id}", response_model=MessageResponse)
async def update_generation(
    generation_id: int,
    data: GenerationUpdate,
    auth: AuthContext = Depends(get_auth)
):
    """Update a generation record."""
    updates = data.model_dump(exclude_unset=True)
    if updates:
        Generation.update(generation_id, **updates)
    return MessageResponse(message="Updated")


@app.delete("/api/generations/{generation_id}", response_model=MessageResponse)
async def delete_generation(generation_id: int, auth: AuthContext = Depends(get_auth)):
    """Delete a generation record."""
    Generation.delete(generation_id)
    return MessageResponse(message="Deleted")


# ============================================================================
# Favorites API
# ============================================================================

@app.get("/api/favorites")
async def list_favorites(
    limit: int = 100,
    offset: int = 0,
    auth: AuthContext = Depends(get_auth)
):
    """List all favorites."""
    favorites = Favorite.list_all(
        user_id=auth.user_id,
        limit=min(limit, 500),
        offset=offset
    )
    return {"favorites": favorites}


@app.post("/api/favorites", response_model=IdResponse, status_code=201)
async def add_favorite(
    data: FavoriteCreate,
    auth: AuthContext = Depends(get_auth)
):
    """Add a generation to favorites."""
    try:
        favorite_id = Favorite.create(
            generation_id=data.generation_id,
            user_id=auth.user_id,
            name=data.name,
            notes=data.notes,
            tags=data.tags
        )
        return IdResponse(id=favorite_id)
    except Exception as e:
        if "UNIQUE constraint" in str(e):
            raise HTTPException(status_code=409, detail="Already favorited")
        raise


@app.delete("/api/favorites/{favorite_id}", response_model=MessageResponse)
async def remove_favorite(favorite_id: int, auth: AuthContext = Depends(get_auth)):
    """Remove from favorites."""
    Favorite.delete(favorite_id)
    return MessageResponse(message="Removed from favorites")


@app.delete("/api/favorites/generation/{generation_id}", response_model=MessageResponse)
async def unfavorite_generation(generation_id: int, auth: AuthContext = Depends(get_auth)):
    """Remove a generation from favorites."""
    Favorite.delete_by_generation(generation_id, auth.user_id)
    return MessageResponse(message="Removed from favorites")


@app.get("/api/favorites/check/{generation_id}")
async def check_favorite(generation_id: int, auth: AuthContext = Depends(get_auth)):
    """Check if a generation is favorited."""
    is_fav = Favorite.is_favorited(generation_id, auth.user_id)
    return {"is_favorited": is_fav}


# ============================================================================
# Voice Profiles API
# ============================================================================

@app.get("/api/voice-profiles")
async def list_voice_profiles(
    model_type: Optional[str] = None,
    auth: AuthContext = Depends(get_auth)
):
    """List voice profiles."""
    profiles = VoiceProfile.list_all(
        user_id=auth.user_id,
        model_type=model_type
    )
    return {"profiles": profiles}


@app.get("/api/voice-profiles/{profile_id}")
async def get_voice_profile(profile_id: int, auth: AuthContext = Depends(get_auth)):
    """Get a voice profile."""
    profile = VoiceProfile.get_by_id(profile_id)
    if not profile:
        raise HTTPException(status_code=404, detail="Profile not found")
    return profile


@app.post("/api/voice-profiles", response_model=IdResponse, status_code=201)
async def create_voice_profile(
    data: VoiceProfileCreate,
    auth: AuthContext = Depends(get_auth)
):
    """Create a voice profile."""
    profile_id = VoiceProfile.create(
        name=data.name,
        model_type=data.model_type,
        config=data.config,
        description=data.description,
        reference_audio_path=data.reference_audio_path,
        user_id=auth.user_id,
        is_default=data.is_default
    )
    return IdResponse(id=profile_id)


@app.patch("/api/voice-profiles/{profile_id}", response_model=MessageResponse)
async def update_voice_profile(
    profile_id: int,
    data: VoiceProfileUpdate,
    auth: AuthContext = Depends(get_auth)
):
    """Update a voice profile."""
    updates = data.model_dump(exclude_unset=True)
    if updates:
        VoiceProfile.update(profile_id, **updates)
    return MessageResponse(message="Updated")


@app.delete("/api/voice-profiles/{profile_id}", response_model=MessageResponse)
async def delete_voice_profile(profile_id: int, auth: AuthContext = Depends(get_auth)):
    """Delete a voice profile."""
    VoiceProfile.delete(profile_id)
    return MessageResponse(message="Deleted")


# ============================================================================
# User Preferences API
# ============================================================================

@app.get("/api/preferences")
async def get_all_preferences(
    category: Optional[str] = None,
    auth: AuthContext = Depends(get_auth)
):
    """Get all preferences."""
    if category:
        prefs = UserPreference.get_category(category, auth.user_id)
    else:
        prefs = UserPreference.get_all(auth.user_id)
    return {"preferences": prefs}


@app.get("/api/preferences/{category}/{key}")
async def get_preference(
    category: str,
    key: str,
    auth: AuthContext = Depends(get_auth)
):
    """Get a specific preference."""
    value = UserPreference.get(category, key, auth.user_id)
    return {"value": value}


@app.put("/api/preferences/{category}/{key}", response_model=MessageResponse)
async def set_preference(
    category: str,
    key: str,
    data: PreferenceValue,
    auth: AuthContext = Depends(get_auth)
):
    """Set a preference value."""
    UserPreference.set(category, key, data.value, auth.user_id)
    return MessageResponse(message="Preference saved")


@app.delete("/api/preferences/{category}/{key}", response_model=MessageResponse)
async def delete_preference(
    category: str,
    key: str,
    auth: AuthContext = Depends(get_auth)
):
    """Delete a preference."""
    UserPreference.delete(category, key, auth.user_id)
    return MessageResponse(message="Preference deleted")


@app.put("/api/preferences/bulk", response_model=MessageResponse)
async def set_bulk_preferences(
    data: BulkPreferences,
    auth: AuthContext = Depends(get_auth)
):
    """Set multiple preferences at once."""
    for category, prefs in data.preferences.items():
        for key, value in prefs.items():
            UserPreference.set(category, key, value, auth.user_id)
    return MessageResponse(message="Preferences saved")


# ============================================================================
# Rescan API
# ============================================================================

@app.post("/api/rescan")
async def rescan_outputs(auth: AuthContext = Depends(get_auth)):
    """Rescan the outputs directory and sync with database."""
    from .rescan import rescan_outputs as do_rescan
    result = do_rescan()
    return result


# ============================================================================
# Statistics API
# ============================================================================

@app.get("/api/stats")
async def get_stats(auth: AuthContext = Depends(get_auth)):
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
                (auth.user_id,),
                fetch_one=True
            )["count"]
        },
        "voice_profiles": {
            "total": execute_query(
                "SELECT COUNT(*) as count FROM voice_profiles WHERE user_id = ?",
                (auth.user_id,),
                fetch_one=True
            )["count"]
        }
    }
    
    return stats


# ============================================================================
# Server Start
# ============================================================================

def start_api_server(host: Optional[str] = None, port: Optional[int] = None):
    """Start the REST API server using Uvicorn."""
    h = host or API_HOST
    p = port or API_PORT
    
    print(f"\nStarting TTS WebUI Database API server...")
    print(f"  • URL: http://{h}:{p}")
    print(f"  • Database: {get_db_path()}")
    print(f"  • Docs: http://{h}:{p}/docs")
    
    uvicorn.run(
        app,
        host=h,
        port=p,
        log_level="info"
    )


if __name__ == "__main__":
    start_api_server()

