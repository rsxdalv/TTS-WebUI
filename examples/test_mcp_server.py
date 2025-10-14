#!/usr/bin/env python3
"""
Example script to test the MCP server manually.

This script sends test requests to the MCP server and displays responses.
It demonstrates how an MCP client would interact with the TTS WebUI MCP server.
"""

import asyncio
import json
from tts_webui.mcp_server.server import MCPServer


async def test_mcp_server():
    """Test the MCP server with various requests."""
    server = MCPServer()
    
    print("=" * 70)
    print("Testing TTS WebUI MCP Server")
    print("=" * 70)
    print()
    
    # Test 1: Initialize
    print("1. Testing initialization...")
    init_request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        }
    }
    response = await server.handle_request(init_request)
    print(f"   Result: {response['result']['serverInfo']}")
    print(f"   Capabilities: {response['result']['capabilities']}")
    print()
    
    # Test 2: List Tools
    print("2. Testing list tools...")
    list_tools_request = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list",
        "params": {}
    }
    response = await server.handle_request(list_tools_request)
    tools = response['result']['tools']
    print(f"   Found {len(tools)} tools:")
    for tool in tools:
        print(f"   - {tool['name']}: {tool['description']}")
    print()
    
    # Test 3: List Resources
    print("3. Testing list resources...")
    list_resources_request = {
        "jsonrpc": "2.0",
        "id": 3,
        "method": "resources/list",
        "params": {}
    }
    response = await server.handle_request(list_resources_request)
    resources = response['result']['resources']
    print(f"   Found {len(resources)} resources:")
    for resource in resources:
        print(f"   - {resource['uri']}: {resource['name']}")
    print()
    
    # Test 4: List Prompts
    print("4. Testing list prompts...")
    list_prompts_request = {
        "jsonrpc": "2.0",
        "id": 4,
        "method": "prompts/list",
        "params": {}
    }
    response = await server.handle_request(list_prompts_request)
    prompts = response['result']['prompts']
    print(f"   Found {len(prompts)} prompts:")
    for prompt in prompts:
        print(f"   - {prompt['name']}: {prompt['description']}")
    print()
    
    # Test 5: Call generate_speech tool
    print("5. Testing generate_speech tool...")
    generate_speech_request = {
        "jsonrpc": "2.0",
        "id": 5,
        "method": "tools/call",
        "params": {
            "name": "generate_speech",
            "arguments": {
                "text": "Hello, world! This is a test of the TTS WebUI MCP server.",
                "model": "maha",
                "language": "english"
            }
        }
    }
    response = await server.handle_request(generate_speech_request)
    result_text = response['result']['content'][0]['text']
    print(f"   Result preview: {result_text[:200]}...")
    print()
    
    # Test 6: Call list_models tool
    print("6. Testing list_models tool...")
    list_models_request = {
        "jsonrpc": "2.0",
        "id": 6,
        "method": "tools/call",
        "params": {
            "name": "list_models",
            "arguments": {}
        }
    }
    response = await server.handle_request(list_models_request)
    models_text = response['result']['content'][0]['text']
    print(f"   {models_text}")
    
    # Test 7: Get a prompt
    print("7. Testing get prompt...")
    get_prompt_request = {
        "jsonrpc": "2.0",
        "id": 7,
        "method": "prompts/get",
        "params": {
            "name": "generate_speech_example",
            "arguments": {
                "text": "Test message"
            }
        }
    }
    response = await server.handle_request(get_prompt_request)
    prompt_content = response['result']['messages'][0]['content']['text']
    print(f"   Prompt preview: {prompt_content[:150]}...")
    print()
    
    # Test 8: Read a resource
    print("8. Testing read resource...")
    read_resource_request = {
        "jsonrpc": "2.0",
        "id": 8,
        "method": "resources/read",
        "params": {
            "uri": "file:///outputs"
        }
    }
    response = await server.handle_request(read_resource_request)
    resource_content = response['result']['contents'][0]['text']
    print(f"   Resource content: {resource_content}")
    print()
    
    print("=" * 70)
    print("All tests completed successfully!")
    print("=" * 70)
    print()
    print("The MCP server is ready to use with MCP clients like Claude Desktop.")
    print()
    print("To use with Claude Desktop, add this to your config:")
    print()
    print(json.dumps({
        "mcpServers": {
            "tts-webui": {
                "command": "tts-webui",
                "args": ["mcp"]
            }
        }
    }, indent=2))


if __name__ == "__main__":
    asyncio.run(test_mcp_server())
