"""
SQLite Database Connection Management
"""

import os
import sqlite3
import threading
from pathlib import Path
from typing import Optional
from contextlib import contextmanager

# Thread-local storage for connections
_local = threading.local()

# Default database path
DEFAULT_DB_PATH = Path("data/sqlite/webui.db")


def get_db_path() -> Path:
    """Get the database file path, creating directory if needed."""
    db_path = Path(os.environ.get("TTS_WEBUI_DB_PATH", DEFAULT_DB_PATH))
    db_path.parent.mkdir(parents=True, exist_ok=True)
    return db_path


def get_db() -> sqlite3.Connection:
    """
    Get a database connection for the current thread.
    Creates a new connection if one doesn't exist.
    """
    if not hasattr(_local, "connection") or _local.connection is None:
        db_path = get_db_path()
        _local.connection = sqlite3.connect(
            str(db_path),
            check_same_thread=False,
            timeout=30.0
        )
        _local.connection.row_factory = sqlite3.Row
        # Enable foreign keys
        _local.connection.execute("PRAGMA foreign_keys = ON")
        # Enable WAL mode for better concurrent access
        _local.connection.execute("PRAGMA journal_mode = WAL")
    return _local.connection


@contextmanager
def get_db_cursor():
    """Context manager for database cursor with automatic commit/rollback."""
    conn = get_db()
    cursor = conn.cursor()
    try:
        yield cursor
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        cursor.close()


def close_db():
    """Close the database connection for the current thread."""
    if hasattr(_local, "connection") and _local.connection is not None:
        _local.connection.close()
        _local.connection = None


def init_db():
    """Initialize the database schema."""
    from .schema import create_tables
    create_tables()
    print(f"Database initialized at: {get_db_path()}")


def execute_query(query: str, params: tuple = (), fetch_one: bool = False, fetch_all: bool = True):
    """
    Execute a query with parameters safely.
    
    Args:
        query: SQL query with ? placeholders
        params: Tuple of parameters
        fetch_one: Return single row
        fetch_all: Return all rows
    
    Returns:
        Query results or lastrowid for INSERT
    """
    with get_db_cursor() as cursor:
        cursor.execute(query, params)
        
        if query.strip().upper().startswith(("INSERT", "UPDATE", "DELETE")):
            return cursor.lastrowid
        
        if fetch_one:
            row = cursor.fetchone()
            return dict(row) if row else None
        
        if fetch_all:
            rows = cursor.fetchall()
            return [dict(row) for row in rows]
        
        return None
