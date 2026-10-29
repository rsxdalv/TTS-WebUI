import { useState, useEffect, useCallback, useRef } from "react";
import { os, filesystem } from "@neutralinojs/lib";
import { WEBUI_ROOT, joinPath } from "./config";

// ---------------------------------------------------------------------------
// Platform helpers
// ---------------------------------------------------------------------------

const PLATFORM = (() => {
  const p = (window as any).NL_OS;
  if (p === "Windows") return "Windows" as const;
  if (p === "Darwin") return "Darwin" as const;
  return "Linux" as const;
})();

type Platform = "Windows" | "Darwin" | "Linux";

// ---------------------------------------------------------------------------
// Lightweight command runner (os.execCommand returns { stdOut, stdErr, exitCode })
// ---------------------------------------------------------------------------

async function execCmd(
  cmd: string,
): Promise<{ stdOut: string; stdErr: string; exitCode: number }> {
  try {
    return await os.execCommand(cmd);
  } catch {
    return { stdOut: "", stdErr: "Command failed to execute", exitCode: 1 };
  }
}

// ---------------------------------------------------------------------------
// Check definitions
// ---------------------------------------------------------------------------

type CheckSeverity = "error" | "warning";
export type CheckStatus = "pending" | "running" | "pass" | "warn" | "fail";

interface CheckResult {
  status: "pass" | "warn" | "fail";
  message?: string;
  /** Label for an optional "fix" button shown alongside the warning/error. */
  fixLabel?: string;
  /** Action to run when the fix button is pressed. */
  onFix?: () => Promise<void>;
}

interface CheckDef {
  id: string;
  title: string;
  severity: CheckSeverity;
  platforms: Platform[];
  run: () => Promise<CheckResult>;
}

// ---------------------------------------------------------------------------
// Individual check implementations
// ---------------------------------------------------------------------------

const CHECK_DEFS: CheckDef[] = [
  // ── Cross-platform ──────────────────────────────────────────────────────
  {
    id: "spaces-in-path",
    title: "No spaces in installation path",
    severity: "error",
    platforms: ["Windows", "Darwin", "Linux"],
    run: async () => {
      if (WEBUI_ROOT.includes(" ")) {
        const example =
          PLATFORM === "Windows"
            ? "e.g. C:\\AI\\tts-webui\\"
            : "e.g. /opt/tts-webui/";
        return {
          status: "fail",
          message: `Installation path contains a space: "${WEBUI_ROOT}". Conda will fail. Please move to a path without spaces — ${example}`,
        };
      }
      return { status: "pass" };
    },
  },

  // ── Windows-only ────────────────────────────────────────────────────────
  {
    id: "system32",
    title: "Not running from System32",
    severity: "error",
    platforms: ["Windows"],
    run: async () => {
      const root = WEBUI_ROOT.toLowerCase();
      if (
        root.includes("windows\\system32") ||
        root.includes("windows/system32")
      ) {
        return {
          status: "fail",
          message:
            "WEBUI_ROOT resolves inside C:\\Windows\\System32. This will cause installation failure. Please move the folder.",
        };
      }
      return { status: "pass" };
    },
  },
  {
    id: "exfat",
    title: "Not on exFAT filesystem",
    severity: "error",
    platforms: ["Windows"],
    run: async () => {
      // Extract drive letter: WEBUI_ROOT looks like C:\Users\...
      const drive = WEBUI_ROOT[0]?.toUpperCase() ?? "C";
      const result = await execCmd(
        `powershell -NoProfile -Command "(Get-PSDrive -Name '${drive}' -ErrorAction SilentlyContinue).FileSystem"`,
      );
      const fs = result.stdOut.trim().toLowerCase();
      if (fs === "exfat") {
        return {
          status: "fail",
          message: `Drive ${drive}: uses exFAT — Conda cannot install on exFAT. Use an NTFS drive (e.g. C:\\AI\\tts-webui\\).`,
        };
      }
      return {
        status: "pass",
        message: fs ? `Filesystem: ${fs.toUpperCase()}` : undefined,
      };
    },
  },
  {
    id: "long-paths",
    title: "Windows long paths enabled",
    severity: "warning",
    platforms: ["Windows"],
    run: async () => {
      const result = await execCmd(
        `powershell -NoProfile -Command "(Get-ItemProperty -Path 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\FileSystem' -Name 'LongPathsEnabled' -ErrorAction SilentlyContinue).LongPathsEnabled"`,
      );
      if (result.stdOut.trim() !== "1") {
        const regFile = joinPath(
          WEBUI_ROOT,
          "installer_scripts",
          "enable_long_paths.reg",
        );
        return {
          status: "warn",
          message:
            "Long paths are not enabled. Installation may fail for deeply nested packages. Import enable_long_paths.reg from installer_scripts and restart.",
          fixLabel: "Enable long paths (import .reg)",
          onFix: async () => {
            // 'reg import' requires elevation; start it via shell so the UAC
            // prompt appears in a new window rather than silently failing.
            await execCmd(`start "" "${regFile}"`);
          },
        };
      }
      return { status: "pass" };
    },
  },
  {
    id: "conda-conflict",
    title: "No conflicting system conda active",
    severity: "warning",
    platforms: ["Windows"],
    run: async () => {
      const condaCheck = await execCmd(`where conda 2>nul`);
      if (condaCheck.exitCode !== 0 || !condaCheck.stdOut.trim()) {
        return { status: "pass", message: "System conda not found (OK)" };
      }
      // Conda is present — check whether the base env is active (marked with *)
      const envCheck = await execCmd(`conda info --envs`);
      const envOutput = envCheck.stdOut + envCheck.stdErr;
      if (envOutput.includes("base") && / \* /.test(envOutput)) {
        return {
          status: "warn",
          message:
            "System conda is installed and the base environment is active. This may interfere with micromamba. Run 'conda deactivate' in a terminal before proceeding.",
        };
      }
      return {
        status: "warn",
        message:
          "System conda is installed (base is not active). It may still interfere — proceed with caution.",
      };
    },
  },
  {
    id: "msvc",
    title: "Visual Studio Build Tools (MSVC) installed",
    severity: "warning",
    platforms: ["Windows"],
    run: async () => {
      // vswhere ships with VS installer and lives at a well-known path.
      const defaultVswhere = `%ProgramFiles(x86)%\\Microsoft Visual Studio\\Installer\\vswhere.exe`;
      const findResult = await execCmd(
        `powershell -NoProfile -Command "if ((Test-Path '${defaultVswhere}') -or (Get-Command vswhere -ErrorAction SilentlyContinue)) { 'found' } else { 'not_found' }"`,
      );

      if (!findResult.stdOut.includes("found")) {
        return {
          status: "warn",
          message:
            "vswhere not found. Cannot verify Visual Studio Build Tools. The installer may fail for Python packages requiring C++ compilation.",
        };
      }

      // vswhere is available — check for installed VS products
      const vsCheck = await execCmd(`vswhere -products * -format json`);
      const output = vsCheck.stdOut.trim();
      if (!output || output === "[]") {
        return {
          status: "warn",
          message:
            "Visual Studio Build Tools (MSVC) is not installed. Some Python packages requiring C++ compilation may fail.",
          fixLabel: "Install via winget (opens new window)",
          onFix: async () => {
            // Open a visible cmd window so the user can follow progress.
            await execCmd(
              `start cmd /k "winget install Microsoft.VisualStudio.2022.BuildTools --silent --override \\"--wait --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended\\""`,
            );
          },
        };
      }
      return { status: "pass", message: "Visual Studio Build Tools found" };
    },
  },

  // ── Linux / macOS ────────────────────────────────────────────────────────
  {
    id: "gcc",
    title: "GCC compiler available",
    severity: "warning",
    platforms: ["Linux", "Darwin"],
    run: async () => {
      const result = await execCmd(`which gcc`);
      if (result.exitCode !== 0 || !result.stdOut.trim()) {
        const hint =
          PLATFORM === "Darwin"
            ? "Install Xcode Command Line Tools: xcode-select --install"
            : "sudo apt update && sudo apt install build-essential";
        return {
          status: "warn",
          message: `gcc is not installed. pip may fail for packages requiring C++ compilation. Fix: ${hint}`,
        };
      }
      return { status: "pass", message: `gcc found: ${result.stdOut.trim()}` };
    },
  },
];

/** The subset of checks relevant to the current platform. */
const RELEVANT_CHECKS = CHECK_DEFS.filter((c) =>
  c.platforms.includes(PLATFORM),
);

/** Sentinel written to disk when all checks have been acknowledged. */
const PREFLIGHT_MARKER = joinPath(
  WEBUI_ROOT,
  "installer_scripts",
  ".preflight_ok",
);

// ---------------------------------------------------------------------------
// UI helpers
// ---------------------------------------------------------------------------

const STATUS_ICON: Record<CheckStatus, string> = {
  pending: "○",
  running: "…",
  pass: "✓",
  warn: "⚠",
  fail: "✗",
};

function statusColor(s: CheckStatus): string {
  if (s === "pass") return "text-green-500 dark:text-green-400";
  if (s === "warn") return "text-yellow-500 dark:text-yellow-400";
  if (s === "fail") return "text-red-500 dark:text-red-400";
  if (s === "running") return "text-blue-400 animate-pulse";
  return "text-gray-400";
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface RunState {
  status: CheckStatus;
  message?: string;
  fixLabel?: string;
}

interface PreflightChecksStepProps {
  onComplete: () => void;
}

export function PreflightChecksStep({ onComplete }: PreflightChecksStepProps) {
  const [states, setStates] = useState<RunState[]>(
    RELEVANT_CHECKS.map(() => ({ status: "pending" })),
  );
  const [allDone, setAllDone] = useState(false);
  const [hasBlockingError, setHasBlockingError] = useState(false);
  const [saving, setSaving] = useState(false);

  // onFix handlers live in a ref to avoid storing functions in React state.
  const fixHandlersRef = useRef<Array<(() => Promise<void>) | undefined>>(
    RELEVANT_CHECKS.map(() => undefined),
  );

  const runAllChecks = useCallback(async () => {
    setAllDone(false);
    setHasBlockingError(false);
    fixHandlersRef.current = RELEVANT_CHECKS.map(() => undefined);
    setStates(RELEVANT_CHECKS.map(() => ({ status: "pending" })));

    let blocking = false;

    for (let i = 0; i < RELEVANT_CHECKS.length; i++) {
      setStates((prev) => {
        const s = [...prev];
        s[i] = { status: "running" };
        return s;
      });

      try {
        const result = await RELEVANT_CHECKS[i].run();
        fixHandlersRef.current[i] = result.onFix;
        setStates((prev) => {
          const s = [...prev];
          s[i] = {
            status: result.status,
            message: result.message,
            fixLabel: result.fixLabel,
          };
          return s;
        });
        if (
          result.status === "fail" &&
          RELEVANT_CHECKS[i].severity === "error"
        ) {
          blocking = true;
        }
      } catch (e: any) {
        fixHandlersRef.current[i] = undefined;
        setStates((prev) => {
          const s = [...prev];
          s[i] = {
            status: "fail",
            message: e?.message ?? "Check failed unexpectedly",
          };
          return s;
        });
        if (RELEVANT_CHECKS[i].severity === "error") blocking = true;
      }
    }

    setHasBlockingError(blocking);
    setAllDone(true);
  }, []);

  useEffect(() => {
    runAllChecks();
  }, [runAllChecks]);

  const handleContinue = async () => {
    setSaving(true);
    try {
      await filesystem.writeFile(PREFLIGHT_MARKER, new Date().toISOString());
    } catch {
      // Non-fatal — the waterfall will re-run this step next time but that's OK.
    }
    setSaving(false);
    onComplete();
  };

  return (
    <div className="space-y-2">
      <p className="text-sm text-gray-500 dark:text-gray-400 mb-3">
        Checking system requirements before installation…
      </p>

      {RELEVANT_CHECKS.map((check, i) => {
        const state = states[i] ?? ({ status: "pending" } as RunState);
        const fixFn = fixHandlersRef.current[i];
        return (
          <div
            key={check.id}
            className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700"
          >
            <div className="flex items-start gap-2">
              <span
                className={`text-base font-mono mt-0.5 flex-shrink-0 w-4 text-center ${statusColor(state.status)}`}
              >
                {STATUS_ICON[state.status]}
              </span>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {check.title}
                </p>
                {state.message && (
                  <p
                    className={`text-xs mt-0.5 break-words ${
                      state.status === "fail"
                        ? "text-red-600 dark:text-red-400"
                        : state.status === "warn"
                          ? "text-yellow-600 dark:text-yellow-400"
                          : "text-gray-500 dark:text-gray-400"
                    }`}
                  >
                    {state.message}
                  </p>
                )}
                {state.fixLabel && fixFn && (
                  <button
                    onClick={fixFn}
                    className="mt-1.5 text-xs py-1 px-3 rounded bg-blue-600 hover:bg-blue-500 text-white transition-colors cursor-pointer"
                  >
                    {state.fixLabel}
                  </button>
                )}
              </div>
            </div>
          </div>
        );
      })}

      {allDone && (
        <div className="mt-4 flex items-center gap-3 flex-wrap">
          <button
            onClick={runAllChecks}
            className="py-2 px-4 rounded-lg bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-900 dark:text-white text-sm transition-colors cursor-pointer"
          >
            Re-run checks
          </button>
          {hasBlockingError ? (
            <p className="text-sm text-red-600 dark:text-red-400">
              Please fix the errors above before continuing.
            </p>
          ) : (
            <button
              onClick={handleContinue}
              disabled={saving}
              className="py-2 px-4 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium transition-colors disabled:opacity-50 cursor-pointer"
            >
              {saving ? "Saving…" : "Continue to Installation"}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
