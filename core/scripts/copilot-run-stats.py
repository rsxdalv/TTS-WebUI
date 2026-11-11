#!/usr/bin/env python3
"""Print turns, tool calls and subagents for this repo's latest Copilot chats.

Reads only event types and counts from VS Code's local transcripts, never chat content.
Usage: python core/scripts/copilot-run-stats.py [number of chats, default 1]
"""
import collections
import json
import os
import sys
from pathlib import Path

repo = Path(__file__).resolve().parents[2].name
count = int(sys.argv[1]) if len(sys.argv) > 1 else 1
home = Path(os.environ.get("APPDATA", Path.home() / ".config")) / "Code" / "User" / "workspaceStorage"

transcripts = []
for ws in home.glob("*"):
    meta = ws / "workspace.json"
    if meta.exists() and repo in meta.read_text(encoding="utf-8", errors="replace"):
        transcripts += (ws / "GitHub.copilot-chat" / "transcripts").glob("*.jsonl")
if not transcripts:
    sys.exit(f"no Copilot transcripts found for {repo}")

for path in sorted(transcripts, key=lambda p: p.stat().st_mtime, reverse=True)[:count]:
    events = collections.Counter()
    tools = collections.Counter()
    for line in path.open(encoding="utf-8", errors="replace"):
        try:
            event = json.loads(line)
        except ValueError:
            continue
        events[event.get("type")] += 1
        if event.get("type") == "tool.execution_start":
            tools[(event.get("data") or {}).get("toolName")] += 1
    subagents = tools["runSubagent"] + tools["execution_subagent"]
    reads = tools["read_file"] + tools["grep_search"] + tools["list_dir"] + tools["file_search"]
    print(
        f"{path.stem[:8]}: {events['assistant.turn_start']} turns, "
        f"{events['tool.execution_start']} tool calls ({reads} reads/searches), {subagents} subagents"
    )
