import { useState } from "react";
import type { ProcessCoreState, ProcessCoreActions } from "./ProcessCore";

interface ProcessDisplayProps {
  name: string;
  openUrl?: string;
  core: ProcessCoreState & ProcessCoreActions;
}

export function ProcessDisplay({ name, openUrl, core }: ProcessDisplayProps) {
  const { outputs, isRunning } = core;
  const [selectedLog, setSelectedLog] = useState(false);

  return (
    <>
      <div className="bg-gray-100 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex flex-col gap-2">
        <h2 className="text-base font-semibold border-b border-gray-200 dark:border-gray-700 pb-2 text-gray-900 dark:text-gray-100">
          {name}
        </h2>
        <button
          onClick={() => (isRunning ? core.terminate() : core.spawn())}
          className={`w-full py-2 px-3 rounded-lg font-medium transition-colors cursor-pointer ${
            isRunning
              ? "bg-red-600 hover:bg-red-500 text-white"
              : "bg-gray-500 hover:bg-gray-600 text-white"
          }`}
        >
          {isRunning ? `Stop ${name}` : `Run ${name}`}
        </button>
        <button
          onClick={() => setSelectedLog(true)}
          className="w-full py-2 px-3 rounded-lg bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-900 dark:text-white font-medium transition-colors cursor-pointer"
        >
          Open logs
        </button>
        {openUrl && (
          <button
            onClick={() => window.open(openUrl, "_blank")}
            className="w-full py-2 px-3 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-medium transition-colors cursor-pointer"
          >
            Open site
          </button>
        )}
      </div>

      {selectedLog && (
        <div
          className="fixed inset-0 bg-black/75 flex items-center justify-center z-50"
          onClick={(e) => {
            if (e.target === e.currentTarget) setSelectedLog(false);
          }}
        >
          <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl p-5 w-[680px] max-w-[90vw] max-h-[80vh] overflow-auto">
            <div className="flex justify-between items-center mb-4 text-gray-900 dark:text-gray-100">
              <span className="font-semibold text-lg">{name} — logs</span>
              <button
                onClick={() => setSelectedLog(false)}
                className="text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white cursor-pointer px-2"
              >
                ✕
              </button>
            </div>
            {outputs.stdout.length > 0 || outputs.stderr.length > 0 ? (
              <>
                <div className="mb-3">
                  <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">
                    stdout
                  </span>
                  <pre className="mt-1 bg-gray-100 dark:bg-black rounded-lg p-3 text-xs overflow-auto max-h-64 whitespace-pre-wrap text-green-700 dark:text-green-400">
                    {outputs.stdout.join("") || "(empty)"}
                  </pre>
                </div>
                <div>
                  <span className="text-xs text-red-400 uppercase tracking-wide">
                    stderr
                  </span>
                  <pre className="mt-1 bg-red-50 dark:bg-red-950 rounded-lg p-3 text-xs overflow-auto max-h-64 whitespace-pre-wrap text-red-700 dark:text-red-300">
                    {outputs.stderr.join("") || "(empty)"}
                  </pre>
                </div>
                {outputs.exitCode !== null && (
                  <p className="mt-2 text-xs text-gray-400 dark:text-gray-500">
                    Exit code: {outputs.exitCode}
                  </p>
                )}
              </>
            ) : (
              <p className="text-gray-500 dark:text-gray-400 text-sm">
                No output yet.
              </p>
            )}
          </div>
        </div>
      )}
    </>
  );
}
