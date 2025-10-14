# MCP Server for TTS WebUI

The TTS WebUI MCP (Model Context Protocol) server allows AI assistants like Claude to interact with text-to-speech functionality directly.

## What is MCP?

The Model Context Protocol (MCP) is a protocol developed by Anthropic that allows AI assistants to connect to external data sources and services. With the TTS WebUI MCP server, you can ask your AI assistant to generate speech from text using various TTS models.

## Features

The TTS WebUI MCP server provides:

### Tools
- **generate_speech**: Convert text to speech using various TTS models
- **list_models**: Get a list of available TTS models and their capabilities
- **list_voices**: List available voices for a specific model
- **get_audio_file**: Get information about generated audio files

### Resources
- **TTS Output Files**: Access to generated audio files
- **Voice Library**: Browse available voice samples and configurations

### Prompts
- **generate_speech_example**: Example workflow for generating speech
- **voice_cloning_example**: Example workflow for voice cloning

## Installation

The MCP server is included with TTS WebUI. Make sure you have TTS WebUI installed:

```bash
pip install -e .
```

## Usage

### Starting the MCP Server

You can start the MCP server using the CLI:

```bash
tts-webui mcp
```

The server will start and listen for MCP requests via stdio (standard input/output).

### Connecting with Claude Desktop

To use the MCP server with Claude Desktop:

1. Locate your Claude Desktop configuration file:
   - **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
   - **Linux**: `~/.config/Claude/claude_desktop_config.json`

2. Add the TTS WebUI MCP server configuration:

```json
{
  "mcpServers": {
    "tts-webui": {
      "command": "tts-webui",
      "args": ["mcp"],
      "description": "TTS WebUI - Text-to-Speech generation"
    }
  }
}
```

3. Restart Claude Desktop

4. You should now see the TTS WebUI tools available when you use Claude

### Connecting with Other MCP Clients

For other MCP clients, configure them to run:

```bash
tts-webui mcp
```

The server uses stdio for communication and follows the MCP protocol specification.

## Example Usage

Once connected, you can ask your AI assistant:

- "Generate speech from the text 'Hello, world!' using the Maha TTS model"
- "List all available TTS models"
- "What voices are available for the Bark model?"
- "Generate a speech file in French saying 'Bonjour le monde'"

## Available Models

The MCP server provides access to these TTS models:

- **Maha TTS**: Multilingual text-to-speech (English, Hindi, Spanish, French, German)
- **Bark**: Text-to-audio with voice cloning (multilingual)
- **Tortoise TTS**: High-quality voice synthesis (English)
- **Vall-E X**: Zero-shot voice cloning (English, Chinese, Japanese)
- **StyleTTS2**: Style-based text-to-speech (English)

And many more models available as extensions.

## Implementation Notes

This is a basic MCP server implementation that provides:

1. **Protocol Compliance**: Follows the MCP 2024-11-05 specification
2. **Stdio Transport**: Uses standard input/output for communication
3. **JSON-RPC 2.0**: Request/response format
4. **Async Operation**: Handles requests asynchronously

### Current Limitations

The current implementation is a foundation that:

- Provides the MCP protocol interface
- Documents available TTS functionality
- Returns placeholder responses for tool calls

To fully integrate with TTS WebUI's generation pipeline, the tool handlers need to be connected to the actual TTS generation functions. This would involve:

1. Importing and calling the actual TTS model functions
2. Managing file paths for generated audio
3. Handling model loading and configuration
4. Implementing proper error handling for TTS operations

## Development

### Running Tests

```bash
pytest tests/test_mcp_server.py
```

### Debugging

Set the log level to DEBUG to see detailed request/response information:

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

## Contributing

Contributions to improve the MCP server are welcome! Areas for enhancement:

- Connect tool handlers to actual TTS generation functions
- Add more sophisticated voice management
- Implement audio file streaming
- Add support for batch generation
- Enhance error handling and validation

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [MCP Python SDK](https://github.com/anthropics/mcp-python-sdk)
- [TTS WebUI Documentation](../README.md)

## License

The MCP server is part of TTS WebUI and follows the same MIT license.
