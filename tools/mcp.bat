@echo off

REM Run activation silently
call conda_env_cmd.bat echo >nul 2>&1

REM Now run the actual MCP server
tts-webui mcp