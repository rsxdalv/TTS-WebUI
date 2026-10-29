# TTS WebUI Ignition

A lightweight Neutralinojs desktop app that launches and manages [TTS WebUI](https://github.com/rsxdalv/tts-webui) — a full-featured text-to-speech generation studio.

## Features

- **One-click launch** of TTS WebUI backend (Gradio) and React frontend
- **Open outputs folder** directly from the app
- **Cross-platform** desktop wrapper using Neutralinojs
- **Extension support** via TTS WebUI's built-in extension marketplace

## Supported Models

Bark, MusicGen, RVC, Tortoise, MAGNeT, Demucs, Maha TTS, Stable Audio, Vocos, MMS, Whisper, Vall-E X, AudioCraft, StyleTTS2, SeamlessM4T, XTTSv2, GPT-SoVITS, Piper TTS, Chatterbox, VibeVoice, Kokoro TTS, DIA, CosyVoice, and many more via extensions.

## Setup

### Prerequisites
- [TTS WebUI](https://github.com/rsxdalv/tts-webui) installed locally
- Neutralinojs CLI: `npm i -g @neutralinojs/neu`

### Steps
1. Clone this repository into a folder (this folder will become your app):
   ```bash
   git clone https://github.com/your-username/tts-webui-ignition.git
   cd tts-webui-ignition
   ```
2. Edit `neutralino.config.json` — set `modes.window.title` and `cli.binaryName`
3. Run `neu update`
4. `cd vite-src`
5. Edit `package.json` — set `name`
6. Create `vite-src/.env.development` with `WEBUI_ROOT=/path/to/tts-webui`
7. `npm install`
8. `cd ..` back to root, then `neu run` to develop

## Development

```bash
neu run
```

## Bundle

From the project root:
```bash
neu build
```

## Configuration

| Variable | Purpose | Default |
|----------|---------|---------|
| `WEBUI_ROOT` | Path to your local TTS WebUI installation | `.` |

Example `vite-src/.env.development`:
```
WEBUI_ROOT=C:/Users/rob/Desktop/tts-generation-webui-main/
```

## How It Works

This app spawns two processes:
1. **TTS WebUI** — Gradio backend at `http://localhost:7770`
2. **React UI** — Frontend at `http://localhost:3000`

The React UI communicates with the Gradio backend to generate audio using any of the supported TTS models.

## License

[MIT](LICENSE)
