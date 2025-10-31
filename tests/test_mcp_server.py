"""Tests for the MCP server."""

import pytest
import json
from tts_webui.mcp_server.server import MCPServer


@pytest.fixture
def mcp_server():
    """Create an MCP server instance for testing."""
    return MCPServer()


@pytest.mark.asyncio
async def test_initialize(mcp_server):
    """Test server initialization."""
    result = await mcp_server.handle_initialize({})
    
    assert "protocolVersion" in result
    assert result["protocolVersion"] == "2024-11-05"
    assert "capabilities" in result
    assert result["capabilities"]["tools"] is True
    assert result["capabilities"]["resources"] is True
    assert result["capabilities"]["prompts"] is True
    assert "serverInfo" in result
    assert result["serverInfo"]["name"] == "tts-webui"


@pytest.mark.asyncio
async def test_list_tools(mcp_server):
    """Test listing available tools."""
    result = await mcp_server.handle_list_tools({})
    
    assert "tools" in result
    tools = result["tools"]
    assert len(tools) == 4
    
    tool_names = [tool["name"] for tool in tools]
    assert "generate_speech" in tool_names
    assert "list_models" in tool_names
    assert "list_voices" in tool_names
    assert "get_audio_file" in tool_names


@pytest.mark.asyncio
async def test_list_resources(mcp_server):
    """Test listing available resources."""
    result = await mcp_server.handle_list_resources({})
    
    assert "resources" in result
    resources = result["resources"]
    assert len(resources) == 2
    
    uris = [resource["uri"] for resource in resources]
    assert "file:///outputs" in uris
    assert "file:///voices" in uris


@pytest.mark.asyncio
async def test_list_prompts(mcp_server):
    """Test listing available prompts."""
    result = await mcp_server.handle_list_prompts({})
    
    assert "prompts" in result
    prompts = result["prompts"]
    assert len(prompts) == 2
    
    prompt_names = [prompt["name"] for prompt in prompts]
    assert "generate_speech_example" in prompt_names
    assert "voice_cloning_example" in prompt_names


@pytest.mark.asyncio
async def test_generate_speech_tool(mcp_server):
    """Test the generate_speech tool."""
    params = {
        "name": "generate_speech",
        "arguments": {
            "text": "Hello, world!",
            "model": "maha",
            "language": "english",
        },
    }
    
    result = await mcp_server.handle_call_tool(params)
    
    assert "content" in result
    assert len(result["content"]) > 0
    assert result["content"][0]["type"] == "text"
    assert "Hello, world!" in result["content"][0]["text"]


@pytest.mark.asyncio
async def test_generate_speech_missing_text(mcp_server):
    """Test generate_speech with missing text parameter."""
    params = {
        "name": "generate_speech",
        "arguments": {},
    }
    
    result = await mcp_server.handle_call_tool(params)
    
    assert "content" in result
    assert result.get("isError") is True
    assert "required" in result["content"][0]["text"].lower()


@pytest.mark.asyncio
async def test_list_models_tool(mcp_server):
    """Test the list_models tool."""
    params = {
        "name": "list_models",
        "arguments": {},
    }
    
    result = await mcp_server.handle_call_tool(params)
    
    assert "content" in result
    content_text = result["content"][0]["text"]
    assert "maha" in content_text.lower()
    assert "bark" in content_text.lower()
    assert "tortoise" in content_text.lower()


@pytest.mark.asyncio
async def test_list_voices_tool(mcp_server):
    """Test the list_voices tool."""
    params = {
        "name": "list_voices",
        "arguments": {
            "model": "maha",
        },
    }
    
    result = await mcp_server.handle_call_tool(params)
    
    assert "content" in result
    assert "maha" in result["content"][0]["text"].lower()


@pytest.mark.asyncio
async def test_unknown_tool(mcp_server):
    """Test calling an unknown tool."""
    params = {
        "name": "unknown_tool",
        "arguments": {},
    }
    
    result = await mcp_server.handle_call_tool(params)
    
    assert "content" in result
    assert result.get("isError") is True
    assert "unknown" in result["content"][0]["text"].lower()


@pytest.mark.asyncio
async def test_handle_request_initialize(mcp_server):
    """Test handling an initialize request."""
    request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {},
    }
    
    response = await mcp_server.handle_request(request)
    
    assert response["jsonrpc"] == "2.0"
    assert response["id"] == 1
    assert "result" in response
    assert response["result"]["protocolVersion"] == "2024-11-05"


@pytest.mark.asyncio
async def test_handle_request_list_tools(mcp_server):
    """Test handling a list tools request."""
    request = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list",
        "params": {},
    }
    
    response = await mcp_server.handle_request(request)
    
    assert response["jsonrpc"] == "2.0"
    assert response["id"] == 2
    assert "result" in response
    assert "tools" in response["result"]


@pytest.mark.asyncio
async def test_handle_request_unknown_method(mcp_server):
    """Test handling an unknown method."""
    request = {
        "jsonrpc": "2.0",
        "id": 3,
        "method": "unknown/method",
        "params": {},
    }
    
    response = await mcp_server.handle_request(request)
    
    assert response["jsonrpc"] == "2.0"
    assert response["id"] == 3
    assert "result" in response
    assert "error" in response["result"]


@pytest.mark.asyncio
async def test_read_resource(mcp_server):
    """Test reading a resource."""
    result = await mcp_server.handle_read_resource({"uri": "file:///outputs"})
    
    assert "contents" in result
    assert len(result["contents"]) > 0
    assert result["contents"][0]["uri"] == "file:///outputs"


@pytest.mark.asyncio
async def test_get_prompt(mcp_server):
    """Test getting a prompt."""
    params = {
        "name": "generate_speech_example",
        "arguments": {
            "text": "Test text",
        },
    }
    
    result = await mcp_server.handle_get_prompt(params)
    
    assert "messages" in result
    assert len(result["messages"]) > 0
    assert result["messages"][0]["role"] == "user"
    assert "Test text" in result["messages"][0]["content"]["text"]


def test_server_creation():
    """Test creating an MCP server instance."""
    server = MCPServer()
    
    assert server.name == "tts-webui"
    assert server.version == "0.0.1"
    assert len(server.tools) == 4
    assert len(server.resources) == 2
    assert len(server.prompts) == 2


def test_server_capabilities():
    """Test server capabilities."""
    server = MCPServer()
    
    assert server.capabilities["tools"] is True
    assert server.capabilities["resources"] is True
    assert server.capabilities["prompts"] is True
