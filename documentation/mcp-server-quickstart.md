# MCP Server Quick Start Guide

Get started with the TTS WebUI MCP server in 5 minutes.

## What You'll Need

- TTS WebUI installed (`pip install -e .`)
- An MCP-compatible client (e.g., Claude Desktop)

## Step 1: Test the Server

First, verify the MCP server works:

```bash
# Run the test script
cd /path/to/TTS-WebUI
PYTHONPATH=. python examples/test_mcp_server.py
```

You should see output showing all server capabilities.

## Step 2: Configure Your MCP Client

### For Claude Desktop

1. **Find your config file:**
   - **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
   - **Linux**: `~/.config/Claude/claude_desktop_config.json`

2. **Add the server configuration:**

```json
{
  "mcpServers": {
    "tts-webui": {
      "command": "tts-webui",
      "args": ["mcp"],
      "description": "Text-to-speech generation with multiple models"
    }
  }
}
```

3. **Restart Claude Desktop**

### For Other MCP Clients

Configure your client to run:
```bash
tts-webui mcp
```

The server communicates via stdio (standard input/output).

## Step 3: Use the Server

Once connected, try these prompts in your MCP client:

### Generate Speech

> "Generate speech from the text 'Hello, world!' using the Maha TTS model"

### List Available Models

> "What TTS models are available?"

### Get Model Information

> "Tell me about the Bark model and what languages it supports"

### List Voices

> "What voices are available for the Maha model?"

## Available Tools

The MCP server provides these tools:

| Tool | Description |
|------|-------------|
| `generate_speech` | Convert text to speech using various TTS models |
| `list_models` | Get a list of available TTS models |
| `list_voices` | List voices for a specific model |
| `get_audio_file` | Get information about generated audio files |

## Available Models

- **Maha TTS**: Multilingual (English, Hindi, Spanish, French, German)
- **Bark**: Voice cloning (Multilingual)
- **Tortoise TTS**: High-quality synthesis (English)
- **Vall-E X**: Zero-shot cloning (English, Chinese, Japanese)
- **StyleTTS2**: Style-based TTS (English)
- And many more available as extensions!

## Troubleshooting

### Server Won't Start

Make sure TTS WebUI is installed:
```bash
pip install -e .
```

### Client Can't Connect

1. Verify the `tts-webui` command is in your PATH
2. Check that your config file is valid JSON
3. Restart your MCP client after configuration changes

### Tool Calls Don't Work

The current implementation provides placeholder responses. To fully integrate with TTS generation:

1. The tool handlers need to be connected to actual TTS functions
2. File paths and model loading need to be configured
3. See [full documentation](./mcp-server.md) for implementation details

## Next Steps

- Read the [full MCP server documentation](./mcp-server.md)
- Explore the [example scripts](../examples/)
- Contribute to connecting the tools to actual TTS generation

## Support

- [GitHub Issues](https://github.com/rsxdalv/TTS-WebUI/issues)
- [Discord Community](https://discord.gg/V8BKTVRtJ9)
- [Documentation](../README.md)
