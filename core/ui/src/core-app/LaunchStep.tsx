// LaunchStep: after all installation steps are done, drives the app lifecycle
// via /api/app/* — no /api/steps/run involvement.
import { useState, useEffect, useRef, useCallback } from "react";
import { client } from "./client";

interface ProcessStatus {
  name: string;
  running: boolean;
  state?: string; // starting, ready, exited, failed, ...
  port?: number;
  error?: string; // why it failed (for example: no Python environment)
  exit_code?: number; // set when it ended by itself
  log_tail?: string[]; // its last output when it ended or failed
  warning?: string; // carried on despite, e.g. a missing GPU label
}

// The parts of the app that can be launched on their own.
const PARTS = [
  { name: "react-ui", label: "Frontend", note: "The React UI", launch: "Launch frontend" },
  { name: "gradio", label: "Gradio", note: "The Gradio tabs and the extensions", launch: "Launch Gradio" },
];

export function LaunchStep() {
  const [processes, setProcesses] = useState<ProcessStatus[]>([]);
  const [appURL, setAppURL] = useState("");
  const [launched, setLaunched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The part (or "all") whose start or stop request is in flight: its buttons wait.
  const [busy, setBusy] = useState<string | null>(null);
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

  // A page opened (or reloaded) while the app is up shows its state at once:
  // the status stream only sends changes, so ask once.
  useEffect(() => {
    let live = true;
    client("/api/app/status")
      .then((r) => (r.ok ? r.json() : []))
      .then((list: ProcessStatus[]) => {
        if (!live || !Array.isArray(list)) return;
        setProcesses(list);
        if (list.some((p) => p.running || p.state === "failed")) {
          setLaunched(true);
          subscribeStatus();
        }
      })
      .catch(() => {});
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
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

  // part is "gradio" or "react-ui"; none starts both.
  async function launch(part?: string) {
    if (busy) return;
    setError(null);
    setBusy(part ?? "all");
    try {
      await launchRequest(part);
    } finally {
      setBusy(null);
    }
  }

  async function launchRequest(part?: string) {
    try {
      const r = await client(`/api/app/start${part ? `?part=${part}` : ""}`, { method: "POST" });
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
    // Show the processes now, not at their next change.
    try {
      const r = await client("/api/app/status");
      if (r.ok) setProcesses((await r.json()) as ProcessStatus[]);
    } catch {
      // the stream will bring the state
    }
  }

  async function stop(part?: string) {
    if (busy) return;
    setBusy(part ?? "all");
    try {
      await client(`/api/app/stop${part ? `?part=${part}` : ""}`, { method: "POST" });
    } finally {
      setBusy(null);
    }
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
  const react = processes.find((p) => p.name === "react-ui");
  const reactURL =
    react?.running && react.port
      ? `${window.location.protocol}//${window.location.hostname}:${react.port}/`
      : "";
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

      <ul className="space-y-2">
        {PARTS.map((part) => {
          const p = processes.find((x) => x.name === part.name);
          if (p?.state === "disabled") return null;
          const running = !!p?.running;
          const ready = running && p?.state === "ready";
          const starting = running && !ready;
          const url =
            part.name === "react-ui"
              ? ready
                ? reactURL
                : ""
              : ready && appURL
                ? appURL
                : "";
          return (
            <li
              key={part.name}
              className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 dark:border-gray-800 px-3 py-2"
            >
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <span
                    className={`w-2 h-2 rounded-full ${
                      ready ? "bg-green-500" : starting ? "bg-amber-400" : "bg-gray-400"
                    }`}
                  />
                  {part.label}
                  <span className="font-normal text-gray-500 dark:text-gray-400">
                    {ready ? "running" : starting ? "starting…" : p?.state === "failed" ? "failed" : "stopped"}
                  </span>
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">{part.note}</p>
                {!running && p?.state === "exited" && (
                  <p className="text-xs text-red-600 dark:text-red-400 mt-0.5 break-words">
                    Ended by itself{p.exit_code !== undefined ? ` (exit code ${p.exit_code})` : ""}.
                    {p.log_tail && p.log_tail.length > 0 ? ` Last output: ${p.log_tail.slice(-3).join(" | ")}` : ""}
                  </p>
                )}
                {p?.error && (
                  <p className="text-xs text-red-600 dark:text-red-400 mt-0.5 break-words">{p.error}</p>
                )}
                {p?.warning && (
                  <p className="text-xs text-amber-700 dark:text-amber-400 mt-0.5">warning: {p.warning}</p>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {url && (
                  <a
                    href={url}
                    target="_blank"
                    rel="noreferrer"
                    className="px-3 py-1.5 rounded bg-green-600 text-white hover:bg-green-700 text-sm"
                  >
                    Open →
                  </a>
                )}
                {running ? (
                  <button
                    onClick={() => stop(part.name)}
                    disabled={!!busy}
                    className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
                  >
                    {busy === part.name ? "Working…" : "Stop"}
                  </button>
                ) : (
                  <button
                    onClick={() => launch(part.name)}
                    disabled={!!busy}
                    className="px-3 py-1.5 rounded bg-blue-600 text-white hover:bg-blue-700 text-sm disabled:opacity-50 disabled:cursor-wait"
                  >
                    {busy === part.name || busy === "all" ? "Starting…" : part.launch}
                  </button>
                )}
              </div>
            </li>
          );
        })}
      </ul>

      {!anyRunning && (
        <button
          onClick={() => launch()}
          disabled={!!busy}
          className="mt-3 px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
        >
          Launch both
        </button>
      )}

      {allStopped && (
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          App stopped.
        </p>
      )}

      {launched && (
        <div className="mt-3 flex gap-2">
          <button
            onClick={() => stop()}
            className="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            Stop all
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
