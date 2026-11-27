// PlatformRows: the storage rows of the Extensions page. They show what the uv
// platform (uv, Python, git, ffmpeg) and the wheel cache take on disk and remove
// them, so an install can be started again from nothing.
import { useCallback, useEffect, useState } from "react";
import { client } from "./client";

interface Row {
  path: string;
  exists: boolean;
  size_bytes: number;
}

interface Storage {
  uv: Row;
  cache: Row;
  venvs: string[];
  app_running: boolean;
}

type Action = "uv" | "cache";

const btn =
  "px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed";
const btnDanger =
  "px-3 py-1 text-sm rounded bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed";

function human(n: number): string {
  if (n >= 1 << 30) return `${(n / (1 << 30)).toFixed(1)} GB`;
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(0)} MB`;
  return `${Math.max(1, Math.round(n / 1024))} KB`;
}

export function PlatformRows({
  installing,
  onChanged,
}: {
  // installing is true while this page runs an install or uninstall.
  installing: boolean;
  onChanged: () => void;
}) {
  const [st, setSt] = useState<Storage | null>(null);
  const [confirm, setConfirm] = useState<Action | null>(null);
  const [working, setWorking] = useState<Action | null>(null);
  const [message, setMessage] = useState<string>("");

  const load = useCallback(async () => {
    try {
      const r = await client("/api/ext/storage");
      if (r.ok) setSt((await r.json()) as Storage);
    } catch {
      // the page's own list shows connection problems
    }
  }, []);

  // Sizes change when an install ends, so look again then.
  useEffect(() => {
    if (!installing) void load();
  }, [installing, load]);

  const run = async (a: Action) => {
    setConfirm(null);
    setWorking(a);
    setMessage("");
    try {
      const path = a === "uv" ? "/api/ext/storage/uv/uninstall" : "/api/ext/storage/cache/clear";
      const r = await client(path, { method: "POST" });
      const body = (await r.json()) as { ok?: boolean; log?: string[]; error?: string };
      setMessage(r.ok ? (body.log ?? []).join("\n") : `Not done: ${body.error ?? `HTTP ${r.status}`}`);
    } catch (e: unknown) {
      setMessage(`Error: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setWorking(null);
      await load();
      onChanged();
    }
  };

  if (!st) return null;
  if (!st.uv.exists && !st.cache.exists && !message) return null;

  const uvBlock = st.app_running
    ? "Stop the app first: its Python lives here."
    : st.venvs.length > 0
      ? `Engines use it: ${st.venvs.join(", ")}. Uninstall them first.`
      : "";
  const disabled = installing || working !== null;

  const row = (
    key: Action,
    name: string,
    what: string,
    r: Row,
    blocked: string,
    verb: string,
  ) =>
    r.exists && (
      <div className="p-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="font-medium">
              {name}{" "}
              <span className="text-xs font-normal text-gray-500 dark:text-gray-400">
                {human(r.size_bytes)}
              </span>
            </div>
            <p className="text-sm text-gray-600 dark:text-gray-400">{what}</p>
            {blocked && <p className="text-xs text-amber-700 dark:text-amber-400">{blocked}</p>}
            <p className="text-xs text-gray-500 break-all">{r.path}</p>
          </div>
          <div className="flex gap-2 shrink-0">
            {confirm === key ? (
              <>
                <button className={btnDanger} onClick={() => run(key)}>
                  {verb}: sure?
                </button>
                <button className={btn} onClick={() => setConfirm(null)}>
                  Cancel
                </button>
              </>
            ) : (
              <button
                className={btn}
                disabled={disabled || !!blocked}
                title={blocked || undefined}
                onClick={() => setConfirm(key)}
              >
                {working === key ? "Working…" : verb}
              </button>
            )}
          </div>
        </div>
      </div>
    );

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
        Disk
      </h2>
      {row(
        "uv",
        "uv platform",
        "uv, Python 3.10, git and ffmpeg that the engines run on. Installed again by the next engine install.",
        st.uv,
        uvBlock,
        "Uninstall",
      )}
      {row(
        "cache",
        "Wheel cache",
        "Downloaded packages (torch and the rest), shared by every engine. Clearing it makes the next install download them again.",
        st.cache,
        "",
        "Clear cache",
      )}
      {message && (
        <pre className="p-2 rounded bg-gray-900 text-gray-100 text-xs whitespace-pre-wrap">{message}</pre>
      )}
    </section>
  );
}
