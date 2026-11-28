// RestartCard: shown after an update replaced files the running launcher still
// uses. Restart asks the launcher to restart itself (with the downloaded program,
// if there is one), waits until it answers again and reloads the page.
import { useState } from "react";
import { client } from "./client";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// bootOf is the id of this start of the launcher ("" when it does not answer).
async function bootOf(): Promise<string> {
  try {
    const r = await fetch("/api/health", { cache: "no-store" });
    if (!r.ok) return "";
    return ((await r.json()) as { boot?: string }).boot ?? "";
  } catch {
    return "";
  }
}

export function RestartCard({ detail }: { detail?: string }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function restart() {
    setBusy(true);
    setError(null);
    const before = await bootOf();
    try {
      const r = await client("/api/launcher/restart", { method: "POST" });
      if (!r.ok) {
        setError(`${r.status}: ${await r.text()}`);
        setBusy(false);
        return;
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
      return;
    }
    // Wait until a launcher with another boot id answers (the restart can be
    // faster than any poll sees it go down), then reload so the page gets the new
    // token and the new UI. After 30 s reload anyway.
    const until = Date.now() + 30_000;
    while (Date.now() < until) {
      await sleep(500);
      const now = await bootOf();
      if (now && now !== before) break;
    }
    window.location.reload();
  }

  return (
    <div className="rounded-xl border border-green-300 dark:border-green-800 bg-green-50 dark:bg-green-950/30 p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="font-medium text-sm">Update installed</div>
          <p className="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
            {detail || "Restart to use the new version. The app is stopped and the launcher comes back by itself."}
          </p>
        </div>
        <button
          onClick={() => void restart()}
          disabled={busy}
          className={`shrink-0 py-1.5 px-3 rounded-lg text-sm font-medium transition-colors cursor-pointer ${
            busy ? "bg-gray-200 dark:bg-gray-700 text-gray-400 cursor-wait" : "bg-blue-600 hover:bg-blue-500 text-white"
          }`}
        >
          {busy ? "Restarting…" : "Restart"}
        </button>
      </div>
      {error && (
        <div className="mt-3 p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-xs font-mono">
          {error}
        </div>
      )}
    </div>
  );
}
