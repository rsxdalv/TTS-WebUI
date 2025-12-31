"""
Database Schema Definition

Tables:
- users: User accounts (for future authentication)
- generations: TTS generation history
- favorites: User favorites (references generations)
- voice_profiles: Voice configuration profiles (JSON)
- user_preferences: User settings and preferences (JSON)
- api_keys: API authentication keys
"""

from .connection import get_db


SCHEMA_VERSION = 1


def create_tables():
    """Create all database tables if they don't exist."""
    conn = get_db()
    cursor = conn.cursor()
    
    # Users table (for future authentication)
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT UNIQUE NOT NULL,
            email TEXT UNIQUE,
            password_hash TEXT,
            is_active BOOLEAN DEFAULT 1,
            is_admin BOOLEAN DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            settings JSON DEFAULT '{}'
        )
    """)
    
    # Create default user if not exists
    cursor.execute("""
        INSERT OR IGNORE INTO users (id, username, email, is_admin)
        VALUES (1, 'default', 'default@localhost', 1)
    """)
    
    # API Keys table for REST API authentication
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS api_keys (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
            key_hash TEXT UNIQUE NOT NULL,
            key_prefix TEXT NOT NULL,
            name TEXT,
            is_active BOOLEAN DEFAULT 1,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            last_used_at TIMESTAMP,
            expires_at TIMESTAMP
        )
    """)
    
    # Generations table - stores TTS generation history
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS generations (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER REFERENCES users(id) ON DELETE SET NULL DEFAULT 1,
            
            -- File information
            filename TEXT NOT NULL,
            filepath TEXT NOT NULL,
            file_exists BOOLEAN DEFAULT 1,
            file_size INTEGER,
            duration_seconds REAL,
            
            -- Generation metadata
            model_name TEXT,
            model_type TEXT,
            text TEXT,
            language TEXT,
            voice TEXT,
            
            -- Full parameters as JSON for flexibility
            parameters JSON DEFAULT '{}',
            
            -- Timing
            generation_time_seconds REAL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            
            -- Status
            status TEXT DEFAULT 'completed',
            error_message TEXT
        )
    """)
    
    # Index for faster lookups
    cursor.execute("""
        CREATE INDEX IF NOT EXISTS idx_generations_created_at 
        ON generations(created_at DESC)
    """)
    cursor.execute("""
        CREATE INDEX IF NOT EXISTS idx_generations_filepath 
        ON generations(filepath)
    """)
    cursor.execute("""
        CREATE INDEX IF NOT EXISTS idx_generations_model 
        ON generations(model_name, model_type)
    """)
    
    # Favorites table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS favorites (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER REFERENCES users(id) ON DELETE CASCADE DEFAULT 1,
            generation_id INTEGER REFERENCES generations(id) ON DELETE CASCADE,
            name TEXT,
            notes TEXT,
            tags JSON DEFAULT '[]',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(user_id, generation_id)
        )
    """)
    
    # Voice profiles table - stores voice configurations
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS voice_profiles (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER REFERENCES users(id) ON DELETE CASCADE DEFAULT 1,
            name TEXT NOT NULL,
            description TEXT,
            model_type TEXT,
            
            -- Voice configuration as JSON (flexible for different models)
            config JSON NOT NULL DEFAULT '{}',
            
            -- Optional reference file
            reference_audio_path TEXT,
            
            is_default BOOLEAN DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    
    # User preferences table - stores UI and app settings
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS user_preferences (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER REFERENCES users(id) ON DELETE CASCADE DEFAULT 1,
            category TEXT NOT NULL,
            key TEXT NOT NULL,
            value JSON NOT NULL,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(user_id, category, key)
        )
    """)
    
    # Schema version tracking
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS schema_version (
            version INTEGER PRIMARY KEY,
            applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    
    cursor.execute("""
        INSERT OR IGNORE INTO schema_version (version) VALUES (?)
    """, (SCHEMA_VERSION,))
    
    conn.commit()
    cursor.close()


def get_schema_version() -> int:
    """Get the current schema version."""
    conn = get_db()
    cursor = conn.cursor()
    cursor.execute("SELECT MAX(version) FROM schema_version")
    row = cursor.fetchone()
    cursor.close()
    return row[0] if row and row[0] else 0
