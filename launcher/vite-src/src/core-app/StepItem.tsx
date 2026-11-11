// StepItem: renders one installation step, calling only /api/steps/*.
import { useState, useEffect, useRef } from "react";
import { client } from "./client";

interface StepItemProps {
  step: string;
  title: string;
  description: string;
  state: "done" | "active" | "pending";
  hasStream: boolean;
  running?: boolean; // server reports a proc is already running (reattach on mount)
  onComplete: () => void;
}

export function StepItem({
  step,
  title,
  description,
  state,
  hasStream,
  running,
  onComplete,
}: StepItemProps) {
  const [isRunning, setIsRunning] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const logEndRef = useRef<HTMLDivElement | null>(null);

  // Auto-scroll to the bottom as new log lines arrive.
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  // Tear down EventSource when step becomes inactive.
  useEffect(() => {
    if (state !== "active") {
      esRef.current?.close();
      esRef.current = null;
    }
  }, [state]);

  // Reattach to an in-progress stream on page reload.
  useEffect(() => {
    if (state === "active" && running) {
      setIsRunning(true);
      openStream();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function openStream() {
    if (esRef.current) return;
    const es = new EventSource(`/api/steps/${step}/stream`);
    esRef.current = es;
    es.onmessage = (e) => setLogs((prev) => [...prev, e.data]);
    es.addEventListener("done", (e) => {
      es.close();
      esRef.current = null;
      const payload = JSON.parse((e as MessageEvent).data ?? "{}") as {
        exitCode?: number;
      };
      if ((payload.exitCode ?? 0) === 0) {
        onComplete();
      } else {
        setError(`Step exited with code ${payload.exitCode}`);
        setIsRunning(false);
      }
    });
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        setIsRunning(false);
      }
    };
  }

  async function runStep() {
    setIsRunning(true);
    setError(null);
    setLogs([]);

    let responseBody: { started?: boolean } = {};
    try {
      const r = await client(`/api/steps/${step}`, { method: "POST" });
      if (!r.ok) {
        const text = await r.text();
        setError(`POST /api/steps/${step} → ${r.status}: ${text}`);
        setIsRunning(false);
        return;
      }
      responseBody = (await r.json()) as { started?: boolean };
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      setIsRunning(false);
      return;
    }

    // If the server says started:false (e.g. update already up-to-date), complete immediately.
    if (hasStream && responseBody.started === false) {
      setIsRunning(false);
      onComplete();
      return;
    }

    if (hasStream) {
      openStream();
    } else {
      setIsRunning(false);
      onComplete();
    }
  }

  return (
    <div
      className={`rounded-xl border p-4 transition-colors ${
        state === "done"
          ? "border-green-300 dark:border-green-800 bg-green-50 dark:bg-green-950/30"
          : state === "active"
            ? "border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900"
            : "border-gray-200 dark:border-gray-800 opacity-50"
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            {state === "done" && (
              <span className="text-green-600 dark:text-green-400 text-sm">✓</span>
            )}
            <span className="font-medium text-sm">{title}</span>
          </div>
          {description && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
              {description}
            </p>
          )}
        </div>
        {state === "active" && (
          <button
            onClick={runStep}
            disabled={isRunning}
            className={`shrink-0 py-1.5 px-3 rounded-lg text-sm font-medium transition-colors cursor-pointer ${
              isRunning
                ? "bg-gray-200 dark:bg-gray-700 text-gray-400 cursor-wait"
                : "bg-blue-600 hover:bg-blue-500 text-white"
            }`}
          >
            {isRunning ? "Running..." : "Run"}
          </button>
        )}
      </div>

      {error && (
        <div className="mt-3 p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-xs font-mono">
          {error}
        </div>
      )}

      {logs.length > 0 && (
        <div className="mt-3 max-h-48 overflow-y-auto bg-gray-950 rounded-lg p-3">
          {logs.map((line, i) => (
            <div key={i} className="text-xs font-mono text-green-400 whitespace-pre-wrap">
              {line}
            </div>
          ))}
          <div ref={logEndRef} />
        </div>
      )}
    </div>
  );
}
