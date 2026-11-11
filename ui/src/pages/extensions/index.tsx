import { useEffect, useState } from "react";
import { client } from "../../api/client.ts";
import type { EngineListEntry } from "../../api/types.ts";

export default function ExtensionsPage() {
  const [engines, setEngines] = useState<EngineListEntry[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function fetchEngines() {
    try {
      const res = await client("/api/engines");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: EngineListEntry[] = await res.json();
      setEngines(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    fetchEngines();
  }, []);

  async function handleStart(id: string) {
    const res = await client(`/api/engines/${id}/start`, { method: "POST" });
    if (!res.ok) {
      setError(`Failed to start ${id}: HTTP ${res.status}`);
      return;
    }
    await fetchEngines();
  }

  async function handleStop(id: string) {
    const res = await client(`/api/engines/${id}/stop`, { method: "POST" });
    if (!res.ok) {
      setError(`Failed to stop ${id}: HTTP ${res.status}`);
      return;
    }
    await fetchEngines();
  }

  return (
    <div>
      <h1>Extensions / Engines</h1>
      {error && <p style={{ color: "red" }}>{error}</p>}
      {engines.length === 0 && !error && <p>No engines registered.</p>}
      <ul>
        {engines.map((e) => (
          <li key={e.id}>
            <strong>{e.id}</strong>{" "}
            {e.status.running ? (
              <span>running (pid {e.status.pid}, port {e.status.port})</span>
            ) : (
              <span>stopped</span>
            )}
            <button onClick={() => handleStart(e.id)} disabled={e.status.running}>
              Start
            </button>
            <button onClick={() => handleStop(e.id)} disabled={!e.status.running}>
              Stop
            </button>
            {/* Logs: GET /api/engines/{id}/logs (SSE) */}
            <small> Logs: /api/engines/{e.id}/logs</small>
          </li>
        ))}
      </ul>
    </div>
  );
}
