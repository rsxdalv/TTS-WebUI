# TTS-WebUI Nodes

A graph-based node editor UI for TTS and audio processing, built with [Rete.js](https://retejs.org/) and React.

## Features

- **Visual Node Editor**: Drag-and-drop interface for creating audio processing pipelines
- **Multiple Node Types**:
  - Audio nodes: Audio Input, Audio Display, Save Audio
  - TTS nodes: OpenAI TTS API integration
  - API nodes: Gradio backend integration
  - Processing nodes: Custom JavaScript execution
  - Input nodes: Text and Number inputs
- **Session Management**:
  - Multiple tabs support
  - LocalStorage persistence
  - Save/Load graphs as JSON files
- **Custom Nodes**: Define custom nodes via JavaScript files
- **React Component UI**: Nodes can contain custom React components

## Getting Started

### Installation

```bash
cd nodes-ui
npm install
```

### Development

```bash
npm run dev
```

The editor will be available at `http://localhost:3002`

### Build

```bash
npm run build
```

### Testing

```bash
npm test
```

## Node Types

### Audio Nodes

- **Audio Input**: Upload or load audio files
- **Audio Display**: Visualize and playback audio
- **Save Audio**: Download audio to file

### TTS Nodes

- **OpenAI TTS**: Generate speech using OpenAI's TTS API
  - Supports models: `tts-1`, `tts-1-hd`
  - Voices: `alloy`, `echo`, `fable`, `onyx`, `nova`, `shimmer`

### API Nodes

- **Gradio API**: Connect to Gradio backend endpoints
  - Integrates with the main TTS-WebUI backend

### Processing Nodes

- **Custom JS**: Execute custom JavaScript code
  - Access inputs via variable names
  - Return outputs as an object

### Input Nodes

- **Text Input**: Input text data
- **Number Input**: Input numeric data

## Socket Types

- **Audio** (green): Audio data (URL, blob, metadata)
- **Text** (blue): String data
- **Number** (yellow): Numeric data

## Graph State

Graphs can be saved and loaded as JSON files with the following structure:

```json
{
  "id": "unique-id",
  "name": "My Graph",
  "nodes": [
    {
      "id": "node-1",
      "type": "AudioInputNode",
      "position": { "x": 100, "y": 200 },
      "controls": { "url": "audio.mp3" }
    }
  ],
  "connections": [
    {
      "id": "conn-1",
      "source": "node-1",
      "sourceOutput": "audio",
      "target": "node-2",
      "targetInput": "audio"
    }
  ],
  "createdAt": "2024-01-01T00:00:00.000Z",
  "updatedAt": "2024-01-01T00:00:00.000Z"
}
```

## Custom Nodes

Custom nodes can be defined in JavaScript files and loaded dynamically:

```javascript
// custom-nodes.js
export const customNodes = [
  {
    id: 'my-custom-node',
    name: 'My Custom Node',
    category: 'Custom',
    inputs: [
      { key: 'input1', label: 'Input 1', socket: 'text' }
    ],
    outputs: [
      { key: 'output', label: 'Output', socket: 'text' }
    ],
    code: `
      return { output: input1.toUpperCase() };
    `
  }
];
```

## Architecture

```
nodes-ui/
├── src/
│   ├── components/     # React components
│   ├── editor/         # Rete.js editor setup
│   ├── hooks/          # React hooks
│   ├── nodes/          # Node definitions
│   ├── pages/          # Next.js pages
│   ├── styles/         # CSS styles
│   └── types/          # TypeScript types
├── public/             # Static assets
└── package.json
```

## Integration with TTS-WebUI

The nodes UI can connect to the main TTS-WebUI backend via the Gradio API nodes. Configure the backend URL in the node settings to connect to your running TTS-WebUI instance.

## License

Same as TTS-WebUI main project.
