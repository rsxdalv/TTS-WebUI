import { useEffect, useState } from "react";
import { client } from "../../api/client.ts";
import type { VersionResponse } from "../../api/types.ts";

export default function SettingsPage() {
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    client("/api/version")
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<VersionResponse>;
      })
      .then((d) => setVersion(d.version))
      .catch(() => setVersion("unknown"));
  }, []);

  return (
    <div>
      <h1>Settings</h1>
      {version !== null && <p>Version: {version}</p>}
      <p>GPU info: — (placeholder)</p>
      <p>Settings coming soon</p>
    </div>
  );
}
