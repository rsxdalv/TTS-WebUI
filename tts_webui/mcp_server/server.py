"""MCP Server implementation for TTS WebUI.

This server exposes TTS functionality through the Model Context Protocol (MCP),
allowing AI assistants to generate speech from text.
"""

import asyncio
import json
import logging
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class MCPServer:
    """MCP Server for TTS WebUI."""

    def __init__(self):
        """Initialize the MCP server."""
        self.name = "tts-webui"
        self.version = "0.0.1"
        self.capabilities = {
            "tools": True,
            "resources": True,
            "prompts": True,
        }
        self.tools = self._register_tools()
        self.resources = self._register_resources()
        self.prompts = self._register_prompts()

    def _register_tools(self) -> List[Dict[str, Any]]:
        """Register available tools."""
        return [
            {
                "name": "generate_speech",
                "description": "Generate speech from text using TTS models",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "text": {
                            "type": "string",
                            "description": "The text to convert to speech",
                        },
                        "model": {
                            "type": "string",
                            "description": "The TTS model to use (e.g., 'maha', 'bark', 'tortoise')",
                            "default": "maha",
                        },
                        "voice": {
                            "type": "string",
                            "description": "The voice/speaker to use",
                            "default": "default",
                        },
                        "language": {
                            "type": "string",
                            "description": "The language for text-to-speech",
                            "default": "english",
                        },
                    },
                    "required": ["text"],
                },
            },
            {
                "name": "list_models",
                "description": "List available TTS models",
                "inputSchema": {
                    "type": "object",
                    "properties": {},
                },
            },
            {
                "name": "list_voices",
                "description": "List available voices for a specific model",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "model": {
                            "type": "string",
                            "description": "The TTS model to get voices for",
                            "default": "maha",
                        },
                    },
                },
            },
            {
                "name": "get_audio_file",
                "description": "Get information about a generated audio file",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "filename": {
                            "type": "string",
                            "description": "The filename of the audio file",
                        },
                    },
                    "required": ["filename"],
                },
            },
        ]

    def _register_resources(self) -> List[Dict[str, Any]]:
        """Register available resources."""
        return [
            {
                "uri": "file:///outputs",
                "name": "TTS Output Files",
                "description": "Generated audio files from TTS",
                "mimeType": "application/json",
            },
            {
                "uri": "file:///voices",
                "name": "Voice Library",
                "description": "Available voice samples and configurations",
                "mimeType": "application/json",
            },
        ]

    def _register_prompts(self) -> List[Dict[str, Any]]:
        """Register available prompts."""
        return [
            {
                "name": "generate_speech_example",
                "description": "Example prompt for generating speech",
                "arguments": [
                    {
                        "name": "text",
                        "description": "Text to convert to speech",
                        "required": True,
                    },
                ],
            },
            {
                "name": "voice_cloning_example",
                "description": "Example prompt for voice cloning",
                "arguments": [
                    {
                        "name": "text",
                        "description": "Text to speak",
                        "required": True,
                    },
                    {
                        "name": "voice_sample",
                        "description": "Path to voice sample file",
                        "required": True,
                    },
                ],
            },
        ]

    async def handle_initialize(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle initialization request."""
        logger.info(f"Initializing MCP server with params: {params}")
        return {
            "protocolVersion": "2024-11-05",
            "capabilities": self.capabilities,
            "serverInfo": {
                "name": self.name,
                "version": self.version,
            },
        }

    async def handle_list_tools(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle list tools request."""
        return {"tools": self.tools}

    async def handle_list_resources(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle list resources request."""
        return {"resources": self.resources}

    async def handle_list_prompts(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle list prompts request."""
        return {"prompts": self.prompts}

    async def handle_call_tool(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle tool call request."""
        tool_name = params.get("name")
        arguments = params.get("arguments", {})

        logger.info(f"Calling tool: {tool_name} with arguments: {arguments}")

        if tool_name == "generate_speech":
            return await self._generate_speech(arguments)
        elif tool_name == "list_models":
            return await self._list_models(arguments)
        elif tool_name == "list_voices":
            return await self._list_voices(arguments)
        elif tool_name == "get_audio_file":
            return await self._get_audio_file(arguments)
        else:
            return {
                "content": [
                    {
                        "type": "text",
                        "text": f"Unknown tool: {tool_name}",
                    }
                ],
                "isError": True,
            }

    async def _generate_speech(self, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """Generate speech from text."""
        text = arguments.get("text", "")
        model = arguments.get("model", "maha")
        voice = arguments.get("voice", "default")
        language = arguments.get("language", "english")

        if not text:
            return {
                "content": [
                    {
                        "type": "text",
                        "text": "Error: 'text' parameter is required",
                    }
                ],
                "isError": True,
            }

        # This is a placeholder - in a real implementation, this would call
        # the actual TTS generation functions
        result_message = f"""Speech generation requested:
- Text: {text[:100]}{'...' if len(text) > 100 else ''}
- Model: {model}
- Voice: {voice}
- Language: {language}

Note: This is a placeholder response. To fully integrate with TTS WebUI,
the server needs to be connected to the actual TTS generation pipeline.
The server would need to call the appropriate TTS model functions and
return the path to the generated audio file.
"""

        return {
            "content": [
                {
                    "type": "text",
                    "text": result_message,
                }
            ],
        }

    async def _list_models(self, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """List available TTS models."""
        models = [
            {
                "name": "maha",
                "description": "Maha TTS - Multilingual text-to-speech",
                "languages": ["english", "hindi", "spanish", "french", "german"],
            },
            {
                "name": "bark",
                "description": "Bark - Text-to-audio model with voice cloning",
                "languages": ["multilingual"],
            },
            {
                "name": "tortoise",
                "description": "Tortoise TTS - High-quality voice synthesis",
                "languages": ["english"],
            },
            {
                "name": "vall_e_x",
                "description": "Vall-E X - Zero-shot voice cloning",
                "languages": ["english", "chinese", "japanese"],
            },
            {
                "name": "styletts2",
                "description": "StyleTTS2 - Style-based text-to-speech",
                "languages": ["english"],
            },
        ]

        models_text = "Available TTS Models:\n\n"
        for model in models:
            models_text += f"• {model['name']}: {model['description']}\n"
            models_text += f"  Languages: {', '.join(model['languages'])}\n\n"

        return {
            "content": [
                {
                    "type": "text",
                    "text": models_text,
                }
            ],
        }

    async def _list_voices(self, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """List available voices for a model."""
        model = arguments.get("model", "maha")

        # This is a placeholder - would query actual voice directories
        voices_text = f"Available voices for {model}:\n\n"
        voices_text += "Note: This is a placeholder. In a full implementation,\n"
        voices_text += "this would scan the voices directory and return actual\n"
        voices_text += "available voice samples and configurations.\n"

        return {
            "content": [
                {
                    "type": "text",
                    "text": voices_text,
                }
            ],
        }

    async def _get_audio_file(self, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """Get information about an audio file."""
        filename = arguments.get("filename", "")

        if not filename:
            return {
                "content": [
                    {
                        "type": "text",
                        "text": "Error: 'filename' parameter is required",
                    }
                ],
                "isError": True,
            }

        # This is a placeholder - would check actual outputs directory
        result_text = f"Audio file information for: {filename}\n\n"
        result_text += "Note: This is a placeholder. In a full implementation,\n"
        result_text += "this would return actual file metadata, duration, format, etc.\n"

        return {
            "content": [
                {
                    "type": "text",
                    "text": result_text,
                }
            ],
        }

    async def handle_read_resource(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle read resource request."""
        uri = params.get("uri", "")

        if uri == "file:///outputs":
            # This is a placeholder - would list actual output files
            content = "Generated audio files would be listed here."
        elif uri == "file:///voices":
            # This is a placeholder - would list actual voice files
            content = "Available voice samples would be listed here."
        else:
            return {
                "contents": [
                    {
                        "uri": uri,
                        "mimeType": "text/plain",
                        "text": f"Unknown resource: {uri}",
                    }
                ],
            }

        return {
            "contents": [
                {
                    "uri": uri,
                    "mimeType": "application/json",
                    "text": content,
                }
            ],
        }

    async def handle_get_prompt(self, params: Dict[str, Any]) -> Dict[str, Any]:
        """Handle get prompt request."""
        name = params.get("name", "")
        arguments = params.get("arguments", {})

        if name == "generate_speech_example":
            text = arguments.get("text", "Hello, world!")
            prompt = f"""To generate speech from text using TTS WebUI:

1. Use the generate_speech tool
2. Provide the text: "{text}"
3. Optionally specify model, voice, and language
4. The system will generate an audio file

Example:
generate_speech(text="{text}", model="maha", language="english")
"""
            return {
                "messages": [
                    {
                        "role": "user",
                        "content": {"type": "text", "text": prompt},
                    }
                ],
            }
        elif name == "voice_cloning_example":
            text = arguments.get("text", "Hello, this is a test.")
            voice_sample = arguments.get("voice_sample", "sample.wav")
            prompt = f"""To clone a voice and generate speech:

1. Prepare a voice sample file: {voice_sample}
2. Use generate_speech with the voice parameter
3. Provide the text: "{text}"
4. The system will clone the voice and generate audio

Example:
generate_speech(text="{text}", model="bark", voice="{voice_sample}")
"""
            return {
                "messages": [
                    {
                        "role": "user",
                        "content": {"type": "text", "text": prompt},
                    }
                ],
            }
        else:
            return {
                "messages": [
                    {
                        "role": "user",
                        "content": {
                            "type": "text",
                            "text": f"Unknown prompt: {name}",
                        },
                    }
                ],
            }

    async def handle_request(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """Handle an incoming MCP request."""
        method = request.get("method")
        params = request.get("params", {})
        request_id = request.get("id")

        logger.info(f"Handling request: {method}")

        try:
            if method == "initialize":
                result = await self.handle_initialize(params)
            elif method == "tools/list":
                result = await self.handle_list_tools(params)
            elif method == "tools/call":
                result = await self.handle_call_tool(params)
            elif method == "resources/list":
                result = await self.handle_list_resources(params)
            elif method == "resources/read":
                result = await self.handle_read_resource(params)
            elif method == "prompts/list":
                result = await self.handle_list_prompts(params)
            elif method == "prompts/get":
                result = await self.handle_get_prompt(params)
            else:
                result = {"error": f"Unknown method: {method}"}

            response = {
                "jsonrpc": "2.0",
                "id": request_id,
                "result": result,
            }
        except Exception as e:
            logger.error(f"Error handling request: {e}", exc_info=True)
            response = {
                "jsonrpc": "2.0",
                "id": request_id,
                "error": {
                    "code": -32603,
                    "message": str(e),
                },
            }

        return response

    async def run(self):
        """Run the MCP server using stdio transport."""
        logger.info("Starting TTS WebUI MCP server...")
        logger.info("Server is ready to accept requests via stdio")

        try:
            while True:
                # Read a line from stdin
                line = await asyncio.get_event_loop().run_in_executor(
                    None, sys.stdin.readline
                )

                if not line:
                    # EOF reached
                    break

                line = line.strip()
                if not line:
                    continue

                try:
                    # Parse the JSON-RPC request
                    request = json.loads(line)
                    logger.debug(f"Received request: {request}")

                    # Handle the request
                    response = await self.handle_request(request)

                    # Send the response
                    response_line = json.dumps(response)
                    print(response_line, flush=True)
                    logger.debug(f"Sent response: {response}")

                except json.JSONDecodeError as e:
                    logger.error(f"Invalid JSON: {e}")
                    error_response = {
                        "jsonrpc": "2.0",
                        "id": None,
                        "error": {
                            "code": -32700,
                            "message": "Parse error",
                        },
                    }
                    print(json.dumps(error_response), flush=True)

        except KeyboardInterrupt:
            logger.info("Server stopped by user")
        except Exception as e:
            logger.error(f"Server error: {e}", exc_info=True)


def create_mcp_server() -> MCPServer:
    """Create and return an MCP server instance."""
    return MCPServer()


async def main():
    """Main entry point for the MCP server."""
    server = create_mcp_server()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main())
