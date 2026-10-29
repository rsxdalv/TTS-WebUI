import { useState } from "react";

const MUSIC_MODELS = ["musicgen", "magnet", "stable-audio", "ace-step"] as const;
type MusicModel = (typeof MUSIC_MODELS)[number];

export default function MusicPage() {
  const [model, setModel] = useState<MusicModel>("musicgen");
  const [prompt, setPrompt] = useState("");
  const [duration, setDuration] = useState(10);

  function handleGenerate() {
    // TODO: wire @gradio/client calls for music generation
    alert(`Generate with ${model}: "${prompt}" for ${duration}s`);
  }

  return (
    <div>
      <h1>Music Generation</h1>
      <div>
        <label htmlFor="music-model">Model</label>
        <select
          id="music-model"
          value={model}
          onChange={(e) => setModel(e.target.value as MusicModel)}
        >
          {MUSIC_MODELS.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor="music-prompt">Prompt</label>
        <textarea
          id="music-prompt"
          rows={3}
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
        />
      </div>
      <div>
        <label htmlFor="music-duration">Duration (seconds)</label>
        <input
          id="music-duration"
          type="number"
          min={1}
          max={300}
          value={duration}
          onChange={(e) => setDuration(Number(e.target.value))}
        />
      </div>
      <button onClick={handleGenerate} disabled={prompt.trim() === ""}>
        Generate
      </button>
    </div>
  );
}
