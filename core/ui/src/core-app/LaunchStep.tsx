// LaunchStep: after all installation steps are done, drives the app lifecycle
// via /api/app/* — no /api/steps/run involvement.
import { useState, useEffect, useRef, useCallback } from "react";
import { client } from "./client";

interface ProcessStatus {
  name: string;
  running: boolean;
}

export function LaunchStep() {
  const [processes, setProcesses] = useState<ProcessStatus[]>([]);
  const [appURL, setAppURL] = useState("");
  const [launched, setLaunched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [diagLines, setDiagLines] = useState<string[]>([]);
  const [diagRunning, setDiagRunning] = useState(false);
  const [lan, setLan] = useState<{ lan: boolean; warning?: string; urls?: string[] }>({ lan: false });
  const esRef = useRef<EventSource | null>(null);
  const diagEsRef = useRef<EventSource | null>(null);
  const linkTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopLinkPolling = useCallback(() => {
    if (linkTimerRef.current !== null) {
      clearInterval(linkTimerRef.current);
      linkTimerRef.current = null;
    }
  }, []);

  const startLinkPolling = useCallback(() => {
    if (linkTimerRef.current !== null) return;
    linkTimerRef.current = setInterval(async () => {
      try {
        const r = await client("/api/app/link");
        if (!r.ok) return;
        const data = (await r.json()) as { url: string };
        if (data.url) setAppURL(data.url);
      } catch {
        // ignore network errors during polling
      }
    }, 2000);
  }, []);

  // Start/stop link polling based on whether any process is running.
  useEffect(() => {
    const anyRunning = processes.some((p) => p.running);
    if (anyRunning) {
      startLinkPolling();
    } else {
      stopLinkPolling();
    }
  }, [processes, startLinkPolling, stopLinkPolling]);

  // LAN mode (TTS_GO_PROXY_LISTEN=lan): show the same warning the launcher logs (R2-8).
  useEffect(() => {
    client("/api/app/lan")
      .then((r) => (r.ok ? r.json() : { lan: false }))
      .then((d) => setLan(d as { lan: boolean; warning?: string; urls?: string[] }))
      .catch(() => {});
  }, []);

  // Cleanup on unmount.
  useEffect(() => {
    return () => {
      esRef.current?.close();
      diagEsRef.current?.close();
      stopLinkPolling();
    };
  }, [stopLinkPolling]);

  function subscribeStatus() {
    if (esRef.current) return;
    const es = new EventSource("/api/app/status/stream");
    esRef.current = es;
    es.onmessage = (e) => {
      try {
        setProcesses(JSON.parse(e.data) as ProcessStatus[]);
      } catch {
        // ignore parse errors
      }
    };
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        esRef.current = null;
      }
    };
  }

  async function launch() {
    setError(null);
    try {
      const r = await client("/api/app/start", { method: "POST" });
      // 409 means already running — subscribe to its status stream.
      if (!r.ok && r.status !== 409) {
        setError(`Failed to start: ${r.status}: ${await r.text()}`);
        return;
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      return;
    }
    setLaunched(true);
    subscribeStatus();
  }

  async function stop() {
    await client("/api/app/stop", { method: "POST" });
  }

  async function restart() {
    setAppURL("");
    await client("/api/app/restart", { method: "POST" });
  }

  async function runDiagnostic() {
    if (diagRunning) return;
    setDiagLines([]);
    setDiagRunning(true);
    diagEsRef.current?.close();
    await client("/api/app/diagnostic", { method: "POST" });
    const es = new EventSource("/api/app/diagnostic/stream");
    diagEsRef.current = es;
    es.onmessage = (e) => {
      setDiagLines((prev) => [...prev, e.data]);
    };
    es.addEventListener("done", () => {
      setDiagRunning(false);
      es.close();
      diagEsRef.current = null;
    });
    es.onerror = () => {
      setDiagRunning(false);
      es.close();
      diagEsRef.current = null;
    };
  }

  const anyRunning = processes.some((p) => p.running);
  const allStopped = launched && processes.length > 0 && !anyRunning;

  return (
    <div className="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <h2 className="text-base font-semibold mb-2">Launch WebUI</h2>

      {lan.lan && (
        <div
          role="alert"
          className="mb-3 p-3 rounded-lg border border-amber-400 bg-amber-50 dark:bg-amber-950/40 dark:border-amber-700 text-sm text-amber-900 dark:text-amber-200"
        >
          <p className="font-medium">Reachable from your network, with no password</p>
          <p className="mt-1">{lan.warning}</p>
          {lan.urls && lan.urls.length > 0 && (
            <ul className="mt-1 font-mono text-xs">
              {lan.urls.map((u) => (
                <li key={u}>{u}</li>
              ))}
            </ul>
          )}
        </div>
      )}

      {error && (
        <p className="text-sm text-red-600 dark:text-red-400 mb-2">{error}</p>
      )}

      {!launched && (
        <button
          onClick={launch}
          className="px-4 py-2 rounded bg-blue-600 text-white hover:bg-blue-700 text-sm"
        >
          Launch WebUI
        </button>
      )}

      {launched && processes.length > 0 && (
        <ul className="mt-2 space-y-1">
          {processes.map((p) => (
            <li key={p.name} className="flex items-center gap-2 text-sm">
              <span
                className={`w-2 h-2 rounded-full ${
                  p.running ? "bg-green-500" : "bg-gray-400"
                }`}
              />
              <span className="font-mono">{p.name}</span>
              <span className="text-gray-500 dark:text-gray-400">
                {p.running ? "running" : "stopped"}
              </span>
            </li>
          ))}
        </ul>
      )}

      {appURL && (
        <a
          href={appURL}
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-flex px-4 py-2 rounded bg-green-600 text-white hover:bg-green-700 text-sm"
        >
          Open WebUI →
        </a>
      )}

      {allStopped && (
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          App stopped.
        </p>
      )}

      {launched && (
        <div className="mt-3 flex gap-2">
          <button
            onClick={stop}
            className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            Stop
          </button>
          <button
            onClick={restart}
            className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            Restart
          </button>
          <button
            onClick={runDiagnostic}
            disabled={diagRunning}
            className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
          >
            {diagRunning ? "Running Diagnostic…" : "Diagnostic"}
          </button>
        </div>
      )}

      {diagLines.length > 0 && (
        <pre className="mt-3 p-2 rounded bg-gray-100 dark:bg-gray-800 text-xs font-mono overflow-auto max-h-48">
          {diagLines.join("\n")}
        </pre>
      )}
    </div>
  );
}
