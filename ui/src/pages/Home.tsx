import { useEffect, useState } from "react";
import { client } from "../api/client.ts";

export default function Home() {
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    client("/api/health")
      .then((r) => r.json())
      .then((data: { ok: boolean; version?: string }) => {
        setVersion(data.version ?? "unknown");
      })
      .catch(() => setVersion("offline"));
  }, []);

  return (
    <main style={{ fontFamily: "sans-serif", padding: "2rem" }}>
      <h1>TTS WebUI</h1>
      <p>Core server: {version === null ? "connecting…" : version}</p>
    </main>
  );
}
