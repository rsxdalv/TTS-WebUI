"""MCP (Model Context Protocol) server for TTS WebUI.

This module provides an MCP server that allows AI assistants to interact
with the TTS WebUI functionality through the Model Context Protocol.
"""

from .server import create_mcp_server

__all__ = ["create_mcp_server"]
