# Examples for TTS WebUI

This directory contains example scripts and usage demonstrations for TTS WebUI.

## MCP Server Examples

### test_mcp_server.py

This script demonstrates how to interact with the TTS WebUI MCP (Model Context Protocol) server programmatically.

**Run the example:**

```bash
# From the project root
cd /path/to/TTS-WebUI
PYTHONPATH=. python examples/test_mcp_server.py
```

**What it does:**

1. Creates an MCP server instance
2. Tests initialization
3. Lists available tools (generate_speech, list_models, list_voices, get_audio_file)
4. Lists available resources (output files, voice library)
5. Lists available prompts (example workflows)
6. Calls the generate_speech tool
7. Calls the list_models tool
8. Gets an example prompt
9. Reads a resource

**Expected output:**

The script will demonstrate all MCP server capabilities and show successful responses for each operation.

### Using with MCP Clients

The MCP server is designed to be used with MCP-compatible clients like Claude Desktop. 

**Configuration example for Claude Desktop:**

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "tts-webui": {
      "command": "tts-webui",
      "args": ["mcp"]
    }
  }
}
```

Once configured, you can ask Claude to:
- "Generate speech from text using Maha TTS"
- "List all available TTS models"
- "What voices are available for the Bark model?"

## More Examples

More examples will be added as the project grows. Check the [documentation](../documentation/) for additional guides and tutorials.
