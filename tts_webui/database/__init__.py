"""
TTS WebUI Database Module

Provides SQLite database functionality for:
- Generation history logging
- User preferences and settings
- Voice profiles
- Favorites management
- User authentication (future)
"""

from .connection import close_db, get_db, init_db
from .decorators import log_generation
from .models import Favorite, Generation, User, UserPreference, VoiceProfile
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
