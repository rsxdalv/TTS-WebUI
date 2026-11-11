import { useEffect, useRef, useState } from "react";
import { client } from "../../api/client.ts";

const TTS_MODELS = [
  "kokoro",
  "chatterbox",
  "bark",
  "tortoise",
  "mms",
  "valle-x",
  "mahatts",
] as const;

type TtsModel = (typeof TTS_MODELS)[number];

export default function TtsPage() {
  const [model, setModel] = useState<TtsModel>("kokoro");
  const [text, setText] = useState("");
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const prevAudioUrl = useRef<string | null>(null);

  // Revoke previous ObjectURL to prevent memory leaks.
  useEffect(() => {
    if (prevAudioUrl.current) URL.revokeObjectURL(prevAudioUrl.current);
    prevAudioUrl.current = audioUrl;
    return () => {
      if (prevAudioUrl.current) URL.revokeObjectURL(prevAudioUrl.current);
    };
  }, [audioUrl]);

  async function handleGenerate() {
    setLoading(true);
    setError(null);
    try {
      // TODO: wire @gradio/client for actual model inference
      const res = await client(`/api/engines/${model}/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text }),
      });
      if (!res.ok) throw new Error(`Generate failed: ${res.status}`);
      const blob = await res.blob();
      setAudioUrl(URL.createObjectURL(blob));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <h1>Text-to-Speech</h1>
      <div>
        <label htmlFor="tts-model">Model</label>
        <select
          id="tts-model"
          value={model}
          onChange={(e) => setModel(e.target.value as TtsModel)}
        >
          {TTS_MODELS.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor="tts-text">Text</label>
        <textarea
          id="tts-text"
          rows={4}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </div>
      <button onClick={handleGenerate} disabled={loading || text.trim() === ""}>
        {loading ? "Generating…" : "Generate"}
      </button>
      {error && <p style={{ color: "red" }}>{error}</p>}
      {audioUrl && <audio controls src={audioUrl} />}
    </div>
  );
}
