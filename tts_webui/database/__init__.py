"""
TTS WebUI Database Module

Provides SQLite database functionality for:
- Generation history logging
- User preferences and settings
- Voice profiles
- Favorites management
- User authentication (future)
"""

from .connection import get_db, init_db, close_db
from .models import Generation, UserPreference, VoiceProfile, User, Favorite
from .decorators import log_generation
from .rescan import rescan_outputs

__all__ = [
    "get_db",
    "init_db", 
    "close_db",
    "Generation",
    "UserPreference",
    "VoiceProfile",
    "User",
    "Favorite",
    "log_generation",
    "rescan_outputs",
]
