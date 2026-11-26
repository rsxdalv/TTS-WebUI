// CoreApp: launcher UI that calls only /api/steps/* and /api/app/*.
// No Neutralino, no NL_* globals, no platform/ imports.
import { useState, useEffect, useCallback } from "react";
import { StepList } from "./StepList";
import { DarkThemeProvider, ThemeToggle } from "../DarkThemeProvider";
import { client } from "./client";
import { ExtensionsView } from "./ExtensionsView";
import { Notice } from "./Notice";

type Page = "setup" | "extensions";

const pageFromHash = (): Page =>
  window.location.hash === "#extensions" ? "extensions" : "setup";

export interface StepStatus {
  step: string;
  done: boolean;
  running?: boolean;
  detail?: string;
}

export function CoreApp() {
  const [statuses, setStatuses] = useState<StepStatus[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState<Page>(pageFromHash);

  useEffect(() => {
    const onHash = () => setPage(pageFromHash());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const fetchStatus = useCallback(async () => {
    try {
      const r = await client("/api/steps/status");
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      setStatuses(await r.json());
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  return (
    <DarkThemeProvider>
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950 text-gray-900 dark:text-gray-100">
        <header className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-6">
            <h1 className="text-lg font-semibold">TTS WebUI</h1>
            <nav className="flex gap-4 text-sm">
              {(["setup", "extensions"] as const).map((p) => (
                <a
                  key={p}
                  href={p === "setup" ? "#" : `#${p}`}
                  className={
                    page === p
                      ? "font-semibold underline underline-offset-4"
                      : "text-gray-500 dark:text-gray-400 hover:underline"
                  }
                >
                  {p === "setup" ? "Setup" : "Extensions"}
                </a>
              ))}
            </nav>
          </div>
          <ThemeToggle />
        </header>
        <main className={`${page === "extensions" ? "max-w-4xl" : "max-w-2xl"} mx-auto px-4 py-8`}>
          <Notice />
          {page === "extensions" && <ExtensionsView />}
          {/* Setup stays mounted while Extensions shows, so the Launch WebUI
              state (running processes, diagnostic output) survives the switch. */}
          <div hidden={page !== "setup"}>
            {error && (
              <div className="mb-6 p-4 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
                Cannot reach Go core: {error}. Is the binary running?
              </div>
            )}
            {statuses === null && !error && (
              <p className="text-gray-500 dark:text-gray-400 text-sm">
                Detecting installation state…
              </p>
            )}
            {statuses !== null && (
              <StepList statuses={statuses} onRefresh={fetchStatus} />
            )}
          </div>
        </main>
      </div>
    </DarkThemeProvider>
  );
}
