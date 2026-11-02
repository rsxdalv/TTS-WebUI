// PreflightView: runs POST /api/steps/preflight, shows check results and fix buttons.
import { useState, useEffect } from "react";

interface PreflightCheck {
  id: string;
  status: "pass" | "warn" | "fail";
  message?: string;
  fix_action?: string;
}

interface PreflightResponse {
  checks: PreflightCheck[];
  ok: boolean;
}

interface FixResponse {
  ok: boolean;
  message?: string;
}

interface PreflightViewProps {
  onComplete: () => void;
}

export function PreflightView({ onComplete }: PreflightViewProps) {
  const [checks, setChecks] = useState<PreflightCheck[]>([]);
  const [ok, setOk] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fixMessages, setFixMessages] = useState<Record<string, string>>({});

  async function runChecks() {
    setLoading(true);
    setFixMessages({});
    try {
      const r = await fetch("/api/steps/preflight", { method: "POST" });
      if (!r.ok) {
        const text = await r.text();
        setChecks([{ id: "error", status: "fail", message: `Server error ${r.status}: ${text}` }]);
        setOk(false);
        return;
      }
      const data = (await r.json()) as PreflightResponse;
      setChecks(data.checks);
      setOk(data.ok);
      if (data.ok) {
        onComplete();
      }
    } catch (e: unknown) {
      setChecks([{ id: "error", status: "fail", message: e instanceof Error ? e.message : String(e) }]);
      setOk(false);
    } finally {
      setLoading(false);
    }
  }

  async function runFix(fixAction: string, id: string) {
    try {
      const r = await fetch(`/api/steps/preflight/fix/${fixAction}`, { method: "POST" });
      const data = (await r.json()) as FixResponse;
      setFixMessages((prev) => ({ ...prev, [id]: data.message ?? (data.ok ? "Fixed!" : "Failed.") }));
    } catch (e: unknown) {
      setFixMessages((prev) => ({ ...prev, [id]: e instanceof Error ? e.message : String(e) }));
    }
  }

  useEffect(() => {
    void runChecks();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const statusIcon = (status: PreflightCheck["status"]) => {
    if (status === "pass") return <span className="text-green-600 dark:text-green-400">✓</span>;
    if (status === "warn") return <span className="text-yellow-500">⚠</span>;
    return <span className="text-red-600 dark:text-red-400">✗</span>;
  };

  const readableId = (id: string) =>
    id.replace(/([A-Z])/g, " $1").replace(/^./, (s) => s.toUpperCase()).trim();

  return (
    <div className="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <div className="font-medium text-sm mb-1">System Pre-flight Checks</div>
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-4">
        Verify system requirements before installation.
      </p>

      {loading && checks.length === 0 && (
        <p className="text-xs text-gray-500 dark:text-gray-400">Running checks…</p>
      )}

      {checks.length > 0 && (
        <div className="flex flex-col gap-3 mb-4">
          {checks.map((c) => (
            <div key={c.id}>
              <div className="flex items-center gap-2 text-sm">
                {statusIcon(c.status)}
                <span className="font-medium">{readableId(c.id)}</span>
              </div>
              {c.message && (
                <p className="ml-5 text-xs text-gray-500 dark:text-gray-400 mt-0.5">{c.message}</p>
              )}
              {fixMessages[c.id] && (
                <p className="ml-5 text-xs text-yellow-600 dark:text-yellow-400 mt-0.5">
                  {fixMessages[c.id]}
                </p>
              )}
              {c.fix_action && (
                <button
                  onClick={() => void runFix(c.fix_action!, c.id)}
                  className="ml-5 mt-1 text-xs px-2 py-1 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 hover:bg-yellow-200 dark:hover:bg-yellow-800 cursor-pointer"
                >
                  Fix
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {!ok && checks.length > 0 && (
        <p className="text-xs text-red-600 dark:text-red-400 mb-3">
          Fix the issues above to continue.
        </p>
      )}

      <button
        onClick={() => void runChecks()}
        disabled={loading}
        className="py-1.5 px-3 rounded-lg text-sm font-medium bg-blue-600 hover:bg-blue-500 disabled:bg-gray-400 text-white transition-colors cursor-pointer"
      >
        {loading ? "Checking…" : "Retry"}
      </button>
    </div>
  );
}
