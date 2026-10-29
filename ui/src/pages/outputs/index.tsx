import { useEffect, useState } from "react";
import { client } from "../../api/client.ts";
import type { GenerationsResponse } from "../../api/types.ts";

export default function OutputsPage() {
  const [files, setFiles] = useState<string[]>([]);
  const [playing, setPlaying] = useState<string | null>(null);

  useEffect(() => {
    client("/api/generations/outputs")
      .then((r): Promise<GenerationsResponse> => {
        if (r.status === 404) return Promise.resolve({ files: [] });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<GenerationsResponse>;
      })
      .then((d) => setFiles(d.files))
      .catch(() => setFiles([]));
  }, []);

  async function handleDelete(filename: string) {
    // TODO: confirm before delete
    const res = await client(
      `/api/generations/outputs/${encodeURIComponent(filename)}/delete`,
      { method: "POST" },
    );
    if (!res.ok) return; // leave UI state unchanged on failure
    setFiles((prev) => prev.filter((f) => f !== filename));
  }

  return (
    <div>
      <h1>Outputs</h1>
      {files.length === 0 && <p>No outputs yet.</p>}
      <ul>
        {files.map((f) => (
          <li key={f}>
            <span
              style={{ cursor: "pointer", textDecoration: "underline" }}
              onClick={() => setPlaying(playing === f ? null : f)}
            >
              {f}
            </span>
            <button onClick={() => handleDelete(f)}>Delete</button>
            {playing === f && (
              <audio
                controls
                autoPlay
                src={`/api/generations/outputs/${encodeURIComponent(f)}`}
              />
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
