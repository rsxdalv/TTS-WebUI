// InstallProgress: the steps of an install with their progress, the elapsed
// time, Cancel while it runs, and the full text log below (open while there are
// no steps, folded away when there are).
import { useEffect, useState } from "react";
import { humanBytes, humanDuration, type StepInfo } from "./install-steps";

const btn =
  "px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed";

// The sweeping bar of a step whose size is not known. Kept here so the page's
// stylesheet needs no change.
const keyframes = `@keyframes tts-indeterminate { 0% { left: -35%; } 100% { left: 100%; } }`;

function Icon({ state }: { state: StepInfo["state"] }) {
  const base = "inline-flex w-5 h-5 shrink-0 items-center justify-center rounded-full text-xs";
  switch (state) {
    case "done":
      return (
        <span className={`${base} bg-green-600 text-white`} aria-label="done">
          ✓
        </span>
      );
    case "skipped":
      return (
        <span className={`${base} bg-gray-400 text-white`} aria-label="skipped">
          –
        </span>
      );
    case "failed":
      return (
        <span className={`${base} bg-red-600 text-white`} aria-label="failed">
          !
        </span>
      );
    case "running":
      return (
        <span
          className={`${base} border-2 border-sky-500 border-t-transparent animate-spin`}
          aria-label="running"
        />
      );
    default:
      return <span className={`${base} border border-gray-400`} aria-label="waiting" />;
  }
}

function Bar({ s }: { s: StepInfo }) {
  const known = !!s.total && s.total > 0 && s.done !== undefined;
  const pct = known ? Math.min(100, Math.round((s.done! * 100) / s.total!)) : 0;
  return (
    <div className="mt-1 ml-7 flex items-center gap-2">
      <div
        className="relative h-2 flex-1 overflow-hidden rounded bg-gray-200 dark:bg-gray-700"
        role="progressbar"
        aria-valuenow={known ? pct : undefined}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        {known ? (
          <div className="h-full bg-sky-500 transition-all duration-500" style={{ width: `${pct}%` }} />
        ) : (
          <div
            className="absolute inset-y-0 w-1/3 bg-sky-500"
            style={{ animation: "tts-indeterminate 1.4s ease-in-out infinite" }}
          />
        )}
      </div>
      {known && (
        <span className="w-52 text-right text-xs tabular-nums text-gray-600 dark:text-gray-400">
          {s.unit
            ? `${s.done} of ${s.total} ${s.unit}`
            : `${s.estimated ? "about " : ""}${pct}% · ${humanBytes(s.done!)} of ${s.estimated ? "about " : ""}${humanBytes(s.total!)}`}
        </span>
      )}
    </div>
  );
}

export function InstallProgress({
  steps,
  log,
  busy,
  onCancel,
  onClose,
}: {
  steps: StepInfo[];
  log: string[];
  busy: boolean;
  onCancel: () => void;
  onClose: () => void;
}) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!busy) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [busy]);

  const doneCount = steps.filter((s) => s.state === "done" || s.state === "skipped").length;
  const failed = steps.some((s) => s.state === "failed");
  const first = steps.find((s) => s.startedAt)?.startedAt;
  const lastFinished = steps.reduce((m, s) => Math.max(m, s.finishedAt ?? 0), 0);
  const allDone = steps.length > 0 && doneCount === steps.length;

  return (
    <div className="flex flex-col gap-3 text-sm">
      <style>{keyframes}</style>
      {steps.length > 0 && (
        <>
          <div className="flex items-baseline justify-between">
            <strong>
              {failed ? "Install failed" : allDone ? "Installed" : busy ? "Installing…" : "Install stopped"}
            </strong>
            <span className="text-xs text-gray-600 dark:text-gray-400 tabular-nums">
              {allDone ? `${steps.length} steps` : `step ${Math.min(doneCount + 1, steps.length)} of ${steps.length}`}
              {first ? ` · ${humanDuration((busy ? now : lastFinished || now) - first)}` : ""}
            </span>
          </div>
          <ol className="flex flex-col gap-2">
            {steps.map((s) => (
              <li key={s.id} className={s.state === "pending" ? "opacity-60" : ""}>
                <div className="flex items-center gap-2">
                  <Icon state={s.state} />
                  <span className="font-medium">{s.label}</span>
                  {s.state === "running" && s.startedAt && (
                    <span className="ml-auto text-xs text-gray-600 dark:text-gray-400 tabular-nums">
                      {humanDuration(now - s.startedAt)}
                    </span>
                  )}
                  {(s.state === "done" || s.state === "failed") && s.startedAt && s.finishedAt && (
                    <span className="ml-auto text-xs text-gray-600 dark:text-gray-400 tabular-nums">
                      {humanDuration(s.finishedAt - s.startedAt)}
                    </span>
                  )}
                  {s.state === "skipped" && (
                    <span className="ml-auto text-xs text-gray-600 dark:text-gray-400">already there</span>
                  )}
                </div>
                {s.state === "running" && <Bar s={s} />}
                {s.detail && s.state !== "pending" && (
                  <p
                    className={`ml-7 mt-0.5 text-xs ${
                      s.state === "failed" ? "text-red-600 dark:text-red-400" : "text-gray-600 dark:text-gray-400"
                    }`}
                  >
                    {s.detail}
                  </p>
                )}
              </li>
            ))}
          </ol>
        </>
      )}

      {log.length > 0 && (
        <details open={steps.length === 0}>
          <summary className="cursor-pointer text-xs text-gray-600 dark:text-gray-400">
            Log ({log.length} lines)
          </summary>
          <pre className="mt-1 p-3 rounded-lg bg-gray-900 text-gray-100 text-xs overflow-auto max-h-60 whitespace-pre-wrap">
            {log.join("\n")}
          </pre>
        </details>
      )}

      <div>
        {busy ? (
          <button className={btn} onClick={onCancel}>
            Cancel
          </button>
        ) : (
          <button className={btn} onClick={onClose}>
            Close
          </button>
        )}
      </div>
    </div>
  );
}
