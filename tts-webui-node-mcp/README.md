# tts-webui-node-mcp

MCP server that forwards requests to the local TTS WebUI Gradio backend.

## Prerequisites

- TTS WebUI Gradio backend running (default http://127.0.0.1:7770/).
- Node.js 18+.

## Setup

1) Install deps: `npm install`
2) Run in dev: `npm run dev`
3) Build: `npm run build`
4) Start compiled server: `npm run start`

## Configuration

- `GRADIO_BACKEND` (or `GRADIO_BACKEND_AUTOMATIC`): backend URL, default `http://127.0.0.1:7770/`.
- `GRADIO_AUTH`: optional `user:pass` for protected Gradio instances.

## Tools

- `tts_generate` – Kokoro TTS (`text`, optional `voice`, `model_name`, `speed`, `seed`, `use_gpu`).
- `music_generate` – Stable Audio (`text`, `negative_prompt`, `seed`, optional `init_audio`, timing and cfg knobs).
- `enhance_audio` – Vocos bandwidth enhancement (`audio`, optional `bandwidth`).
- `stt_transcribe` – Whisper STT (`audio`, optional `model_name`).

File inputs accept absolute/relative paths, http(s) URLs, or data URIs.
