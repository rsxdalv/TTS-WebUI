import { useState, useEffect, useCallback, ReactNode } from "react";
import { filesystem } from "@neutralinojs/lib";
import { WEBUI_ROOT, joinPath } from "./config";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type ConditionFn = () => Promise<boolean>;

export interface WaterfallStepDef {
  id: string;
  title: string;
  description?: string;
  /**
   * Async predicate — return `true` if this step is already complete and can
   * be skipped.  Evaluated on mount and after every step completion.
   */
  condition: ConditionFn;
  /**
   * Return the UI for this step.  Call `onComplete()` when the step succeeds
   * so the waterfall advances to the next one.
   */
  render: (onComplete: () => void) => ReactNode;
}

// ---------------------------------------------------------------------------
// Condition helpers
// ---------------------------------------------------------------------------

/** Returns `true` when a file or directory exists (uses filesystem.getStats). */
export async function pathExists(path: string): Promise<boolean> {
  try {
    await filesystem.getStats(path);
    return true;
  } catch {
    return false;
  }
}

/**
 * Like `pathExists` but resolves `relativePath` against `WEBUI_ROOT`.
 * Use this for all installation-state checks so the path is always
 * anchored to the correct root directory.
 */
export function pathExistsInRoot(relativePath: string): Promise<boolean> {
  return pathExists(joinPath(WEBUI_ROOT, relativePath));
}

/** Returns `true` when an HTTP endpoint responds with a 2xx status. */
export async function httpReachable(
  url: string,
  timeoutMs = 2000,
): Promise<boolean> {
  try {
    const resp = await fetch(url, { signal: AbortSignal.timeout(timeoutMs) });
    return resp.ok;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

type StepStatus = "pending" | "checking" | "active" | "completed";

// ---------------------------------------------------------------------------
// WaterfallInstaller
// ---------------------------------------------------------------------------

export interface WaterfallInstallerProps {
  steps: WaterfallStepDef[];
}

/**
 * Renders an ordered list of installation steps.  On mount it runs each
 * step's `condition` function in order and stops at the first step that
 * isn't yet complete, marking it as the active step.  Steps before it are
 * shown as completed; steps after it are dimmed.
 *
 * When `onComplete()` is called from inside a step's `render` function the
 * waterfall re-evaluates subsequent conditions and advances to the next
 * incomplete step.
 */
export function WaterfallInstaller({ steps }: WaterfallInstallerProps) {
  const [statuses, setStatuses] = useState<StepStatus[]>(
    steps.map(() => "pending"),
  );
  const [ready, setReady] = useState(false);

  /** Run condition checks from `from` onward; mark the first `false` as active. */
  const runDetectionFrom = useCallback(
    async (from: number) => {
      for (let i = from; i < steps.length; i++) {
        setStatuses((prev) => {
          const s = [...prev];
          s[i] = "checking";
          return s;
        });
        let done = false;
        try {
          done = await steps[i].condition();
        } catch {
          done = false;
        }

        if (done) {
          setStatuses((prev) => {
            const s = [...prev];
            s[i] = "completed";
            return s;
          });
        } else {
          // This is the active step; everything after is still pending.
          setStatuses((prev) => {
            const s = [...prev];
            s[i] = "active";
            for (let j = i + 1; j < s.length; j++) s[j] = "pending";
            return s;
          });
          return;
        }
      }
      // All steps completed — nothing left to do.
    },
    [steps],
  );

  useEffect(() => {
    runDetectionFrom(0).then(() => setReady(true));
  }, [runDetectionFrom]);

  const handleComplete = useCallback(
    (i: number) => {
      setStatuses((prev) => {
        const s = [...prev];
        s[i] = "completed";
        return s;
      });
      runDetectionFrom(i + 1);
    },
    [runDetectionFrom],
  );

  const handleRerun = useCallback((i: number) => {
    setStatuses((prev) => {
      const s = [...prev];
      s[i] = "active";
      for (let j = i + 1; j < s.length; j++) {
        s[j] = "pending";
      }
      return s;
    });
  }, []);

  if (!ready) {
    return (
      <div className="flex items-center gap-2 py-8 justify-center text-gray-500 dark:text-gray-400">
        <div className="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full" />
        <span className="text-sm">Detecting installation state…</span>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {steps.map((step, i) => (
        <WaterfallStepCard
          key={step.id}
          index={i}
          step={step}
          status={statuses[i]}
          onComplete={() => handleComplete(i)}
          onRerun={() => handleRerun(i)}
        />
      ))}
    </div>
  );
}

const devMode = import.meta.env.VITE_DEV_MODE;

// ---------------------------------------------------------------------------
// WaterfallStepCard
// ---------------------------------------------------------------------------

interface WaterfallStepCardProps {
  index: number;
  step: WaterfallStepDef;
  status: StepStatus;
  onComplete: () => void;
  onRerun: () => void;
}

function WaterfallStepCard({
  index,
  step,
  status,
  onComplete,
  onRerun,
}: WaterfallStepCardProps) {
  const isCompleted = status === "completed";
  const isActive = status === "active";
  const isChecking = status === "checking";
  const isPending = status === "pending";

  return (
    <div
      className={`rounded-xl border-2 transition-all duration-200 overflow-hidden ${
        isActive
          ? "border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900"
          : isCompleted
            ? "border-green-500/50 dark:border-green-700/50 bg-green-50 dark:bg-green-950/30"
            : "border-dashed border-gray-300 dark:border-gray-700 opacity-50"
      }`}
    >
      {/* ---- Header ---- */}
      <div className="flex items-center gap-3 px-4 py-3">
        {/* Step indicator bubble */}
        <div
          className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold shrink-0 select-none ${
            isCompleted
              ? "bg-green-500 text-white"
              : isActive
                ? "bg-blue-500 text-white"
                : isChecking
                  ? "bg-yellow-400 text-black animate-pulse"
                  : "bg-gray-300 dark:bg-gray-700 text-gray-500 dark:text-gray-400"
          }`}
        >
          {isCompleted ? "✓" : isChecking ? "…" : index + 1}
        </div>

        {/* Title + description */}
        <div className="flex-1 min-w-0">
          <p
            className={`font-semibold text-sm ${
              isActive
                ? "text-gray-900 dark:text-gray-100"
                : isCompleted
                  ? "text-green-700 dark:text-green-400"
                  : "text-gray-400 dark:text-gray-600"
            }`}
          >
            {step.title}
          </p>
          {step.description && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
              {step.description}
            </p>
          )}
        </div>

        {/* Right-side badge */}
        {isCompleted && (
          <button
            onClick={onRerun}
            className="text-xs text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors cursor-pointer"
            title="Re-run this step"
          >
            ↩ re-run
          </button>
        )}
        {/* Skip */}
        {
          devMode && isActive && (
            <button
              onClick={onComplete}
              className="text-xs text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors cursor-pointer"
              title="Skip this step"
            >
              skip →
            </button>
          )
        }
        {isPending && (
          <span className="text-xs text-gray-400 dark:text-gray-600">
            waiting
          </span>
        )}
      </div>
      {/* ---- Body (only when active) ---- */}
      {(isActive || (isCompleted && devMode)) && (
        <div className={`px-4 pb-4 border-t border-blue-200 dark:border-blue-900/50 pt-3 ${isCompleted ? "hidden" : ""}`}>
          {step.render(onComplete)}
        </div>
      )}
    </div>
  );
}
