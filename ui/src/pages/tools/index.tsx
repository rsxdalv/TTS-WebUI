import { useEffect, useState } from "react";
import { client } from "../../api/client.ts";
import type { VersionResponse } from "../../api/types.ts";

const TOOL_CATEGORIES = [
  "Audio Splitter",
  "Pitch Shift",
  "GPU Info",
  "API Config",
] as const;

export default function ToolsPage() {
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    client("/api/version")
      .then((r): Promise<VersionResponse> => r.json())
      .then((d) => setVersion(d.version))
      .catch(() => setVersion("unknown"));
  }, []);

  return (
    <div>
      <h1>Tools</h1>
      {version !== null && <p>Version: {version}</p>}
      <ul>
        {TOOL_CATEGORIES.map((cat) => (
          <li key={cat}>
            {/* TODO: add sub-routing when tool pages are implemented */}
            <span>{cat}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
