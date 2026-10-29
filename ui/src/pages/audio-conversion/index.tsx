import { useState } from "react";
import { client } from "../../api/client.ts";
import type { UploadResponse } from "../../api/types.ts";

const CONVERSION_MODELS = ["rvc", "demucs", "vocos"] as const;
type ConversionModel = (typeof CONVERSION_MODELS)[number];

export default function AudioConversionPage() {
  const [model, setModel] = useState<ConversionModel>("rvc");
  const [uploaded, setUploaded] = useState<UploadResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setLoading(true);
    setError(null);
    try {
      const form = new FormData();
      form.append("file", file);
      const res = await client("/api/upload", { method: "POST", body: form });
      if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
      const data: UploadResponse = await res.json();
      setUploaded(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  function handleConvert() {
    // TODO: wire actual conversion API for each model
    alert(`Convert with ${model}: ${uploaded?.path}`);
  }

  return (
    <div>
      <h1>Audio Conversion</h1>
      <div>
        <label htmlFor="conv-model">Model</label>
        <select
          id="conv-model"
          value={model}
          onChange={(e) => setModel(e.target.value as ConversionModel)}
        >
          {CONVERSION_MODELS.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor="conv-file">Upload audio</label>
        <input
          id="conv-file"
          type="file"
          accept="audio/*"
          onChange={handleUpload}
          disabled={loading}
        />
      </div>
      {loading && <p>Uploading…</p>}
      {error && <p style={{ color: "red" }}>{error}</p>}
      {uploaded && (
        <div>
          <p>Uploaded: {uploaded.path}</p>
          <button onClick={handleConvert}>Convert</button>
        </div>
      )}
    </div>
  );
}
