// CoreApp: launcher UI that calls only /api/steps/* and /api/app/*.
// No Neutralino, no NL_* globals, no platform/ imports.
import { useState, useEffect, useCallback } from "react";
import { StepList } from "./StepList";
import { DarkThemeProvider, ThemeToggle } from "../DarkThemeProvider";
import { client } from "./client";

export interface StepStatus {
  step: string;
  done: boolean;
  running?: boolean;
}

export function CoreApp() {
  const [statuses, setStatuses] = useState<StepStatus[] | null>(null);
  const [error, setError] = useState<string | null>(null);

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
          <h1 className="text-lg font-semibold">TTS WebUI Setup</h1>
          <ThemeToggle />
        </header>
        <main className="max-w-2xl mx-auto px-4 py-8">
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
        </main>
      </div>
    </DarkThemeProvider>
  );
}
