# MCP Server Implementation Details

This document provides technical details about the TTS WebUI MCP server implementation.

## Architecture

### Overview

The MCP server follows a layered architecture:

```
┌─────────────────────────────────────┐
│   MCP Client (Claude Desktop, etc)  │
└──────────────┬──────────────────────┘
               │ stdio (JSON-RPC 2.0)
┌──────────────▼──────────────────────┐
│      MCP Server (server.py)         │
│  ┌───────────────────────────────┐  │
│  │  Protocol Handler             │  │
│  │  - initialize                 │  │
│  │  - tools/list, tools/call     │  │
│  │  - resources/list, read       │  │
│  │  - prompts/list, get          │  │
│  └───────────┬───────────────────┘  │
│              │                       │
│  ┌───────────▼───────────────────┐  │
│  │  Tool Implementations         │  │
│  │  - generate_speech            │  │
│  │  - list_models                │  │
│  │  - list_voices                │  │
│  │  - get_audio_file             │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   TTS WebUI Core (future)           │
│   - Model loading                   │
│   - Audio generation                │
│   - File management                 │
└─────────────────────────────────────┘
```

### Components

#### 1. MCPServer Class

The main server class that handles all MCP protocol operations.

**Key Responsibilities:**
- Protocol negotiation and initialization
- Request routing and handling
- Tool, resource, and prompt registration
- Error handling and logging

**Methods:**
- `handle_initialize()`: Protocol initialization
- `handle_list_tools()`: Tool discovery
- `handle_call_tool()`: Tool execution
- `handle_list_resources()`: Resource discovery
- `handle_read_resource()`: Resource access
- `handle_list_prompts()`: Prompt discovery
- `handle_get_prompt()`: Prompt retrieval
- `handle_request()`: Main request dispatcher

#### 2. Communication Layer

Uses **stdio transport** for communication:
- Reads JSON-RPC requests from stdin
- Writes JSON-RPC responses to stdout
- Line-based protocol (one request/response per line)
- Async I/O for non-blocking operation

#### 3. Tool Implementations

Each tool is implemented as an async method:

```python
async def _generate_speech(self, arguments: Dict[str, Any]) -> Dict[str, Any]:
    """Generate speech from text."""
    # Validate arguments
    # Call TTS functions (placeholder in current implementation)
    # Return result in MCP format
```

## Protocol Details

### Message Format

All messages use JSON-RPC 2.0 format:

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "generate_speech",
    "arguments": {
      "text": "Hello, world!",
      "model": "maha"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Speech generated successfully"
      }
    ]
  }
}
```

**Error Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Internal error"
  }
}
```

### Supported Methods

| Method | Description |
|--------|-------------|
| `initialize` | Initialize connection and negotiate capabilities |
| `tools/list` | List all available tools |
| `tools/call` | Execute a tool with arguments |
| `resources/list` | List all available resources |
| `resources/read` | Read a specific resource |
| `prompts/list` | List all available prompts |
| `prompts/get` | Get a specific prompt with arguments |

## Tool Specifications

### generate_speech

Convert text to speech using TTS models.

**Input Schema:**
```typescript
{
  text: string;          // Required: Text to convert
  model?: string;        // Optional: TTS model (default: "maha")
  voice?: string;        // Optional: Voice/speaker (default: "default")
  language?: string;     // Optional: Language (default: "english")
}
```

**Output:**
```typescript
{
  content: [
    {
      type: "text",
      text: string;      // Result description or audio file path
    }
  ]
}
```

### list_models

List all available TTS models.

**Input Schema:**
```typescript
{}  // No parameters
```

**Output:**
```typescript
{
  content: [
    {
      type: "text",
      text: string;      // Formatted list of models
    }
  ]
}
```

### list_voices

List available voices for a model.

**Input Schema:**
```typescript
{
  model?: string;        // Optional: Model name (default: "maha")
}
```

**Output:**
```typescript
{
  content: [
    {
      type: "text",
      text: string;      // Formatted list of voices
    }
  ]
}
```

### get_audio_file

Get information about a generated audio file.

**Input Schema:**
```typescript
{
  filename: string;      // Required: Audio filename
}
```

**Output:**
```typescript
{
  content: [
    {
      type: "text",
      text: string;      // File metadata
    }
  ]
}
```

## Integration Points

### Current Implementation (Placeholder)

The current implementation provides:
- ✅ Complete MCP protocol handling
- ✅ Tool definitions and schemas
- ✅ Resource and prompt management
- ✅ Error handling and validation
- ⚠️ Placeholder responses (not connected to actual TTS)

### Future Integration

To fully integrate with TTS WebUI:

1. **Import TTS Functions**
   ```python
   from tts_webui.maha_tts import generate_maha_tts
   from tts_webui.bark import generate_bark
   # etc.
   ```

2. **Call TTS Generation**
   ```python
   async def _generate_speech(self, arguments):
       text = arguments.get("text")
       model = arguments.get("model", "maha")
       
       # Call actual TTS function
       if model == "maha":
           audio_file = await generate_maha_tts(text, ...)
       elif model == "bark":
           audio_file = await generate_bark(text, ...)
       
       return {
           "content": [{
               "type": "text",
               "text": f"Audio generated: {audio_file}"
           }]
       }
   ```

3. **Manage File Paths**
   - Use `tts_webui.utils.outputs.path` for output management
   - Handle audio file storage and retrieval
   - Implement file cleanup policies

4. **Resource Implementation**
   - Scan output directory for generated files
   - Scan voices directory for available voices
   - Return file lists and metadata

## Error Handling

The server implements comprehensive error handling:

1. **Request Parsing Errors** (JSON-RPC -32700)
   - Invalid JSON
   - Malformed requests

2. **Method Not Found** (-32601)
   - Unknown methods
   - Unsupported operations

3. **Invalid Parameters** (-32602)
   - Missing required arguments
   - Invalid argument types

4. **Internal Errors** (-32603)
   - Tool execution failures
   - Server exceptions

## Testing

The test suite (`tests/test_mcp_server.py`) covers:

- Protocol initialization
- Tool listing and execution
- Resource management
- Prompt handling
- Error scenarios
- Request/response format validation

Run tests:
```bash
pytest tests/test_mcp_server.py -v
```

## Performance Considerations

1. **Async Operation**
   - Non-blocking I/O
   - Concurrent request handling (if needed)
   - Proper resource cleanup

2. **Memory Management**
   - Streaming for large files (future)
   - Efficient audio data handling
   - Resource pooling for models

3. **Logging**
   - Configurable log levels
   - Request/response debugging
   - Performance monitoring

## Security

Current considerations:

1. **Input Validation**
   - Parameter type checking
   - Length limits on text input
   - Path traversal prevention

2. **Resource Access**
   - Limited to configured directories
   - No arbitrary file access
   - Sandboxed execution

3. **Authentication**
   - Currently none (runs locally)
   - Future: Token-based auth option
   - Transport security (stdio is local)

## Extending the Server

### Adding New Tools

1. Define tool specification in `_register_tools()`
2. Implement tool handler method
3. Add to `handle_call_tool()` dispatcher
4. Write tests
5. Update documentation

Example:
```python
def _register_tools(self):
    return [
        # ... existing tools ...
        {
            "name": "convert_audio",
            "description": "Convert audio format",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "input_file": {"type": "string"},
                    "output_format": {"type": "string"}
                },
                "required": ["input_file", "output_format"]
            }
        }
    ]

async def _convert_audio(self, arguments):
    # Implementation
    pass
```

### Adding New Resources

1. Define resource in `_register_resources()`
2. Implement read handler in `handle_read_resource()`
3. Add tests

### Adding New Prompts

1. Define prompt in `_register_prompts()`
2. Implement prompt handler in `handle_get_prompt()`
3. Add tests

## References

- [MCP Specification](https://modelcontextprotocol.io/)
- [JSON-RPC 2.0](https://www.jsonrpc.org/specification)
- [Python asyncio](https://docs.python.org/3/library/asyncio.html)

## Contributing

Contributions to improve the MCP server are welcome:

1. Connect tools to actual TTS functions
2. Implement resource scanning
3. Add more TTS models
4. Improve error handling
5. Add authentication
6. Performance optimizations

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.
