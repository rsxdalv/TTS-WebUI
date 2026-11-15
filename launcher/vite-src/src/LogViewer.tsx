import { useState, useEffect, useRef } from "react";

// Installer server runs on port 7771 — see server.js in installer_scripts/js
const INSTALLER_URL = "http://localhost:7771";

interface InstallerState {
  status:
    | "initializing"
    | "checking_dependencies"
    | "updating_repo"
    | "installing"
    | "ready"
    | "error";
  currentStep: number;
  totalSteps: number;
  condaReady: boolean;
  torchReady: boolean;
  reactUIReady: boolean;
  lastError: string | null;
  gitHash: string | null;
  timestamp: number;
}

interface PollResponse {
  messages?: string[];
  state?: InstallerState;
}

const STATUS_COLORS: Record<InstallerState["status"], string> = {
  initializing: "bg-blue-500",
  checking_dependencies: "bg-green-500",
  updating_repo: "bg-yellow-500",
  installing: "bg-orange-500",
  ready: "bg-cyan-500",
  error: "bg-orange-500",
};

const STATUS_LABELS: Record<InstallerState["status"], string> = {
  initializing: "Initializing",
  checking_dependencies: "Checking Dependencies",
  updating_repo: "Updating Repo",
  installing: "Installing",
  ready: "Ready",
  error: "Error",
};

export function LogViewer() {
  const [messages, setMessages] = useState<string[]>([]);
  const [installerState, setInstallerState] = useState<InstallerState>({
    status: "initializing",
    currentStep: 0,
    totalSteps: 5,
    condaReady: false,
    torchReady: false,
    reactUIReady: false,
    lastError: null,
    gitHash: null,
    timestamp: 0,
  });

  const xhrRef = useRef<XMLHttpRequest | null>(null);

  const pollState = () => {
    fetch(`${INSTALLER_URL}/poll`)
      .then((response) => response.json())
      .then((data: PollResponse) => {
        if (data.messages && data.messages.length > 0) {
          setMessages(data.messages);
        }
        if (data.state) {
          setInstallerState(data.state);
        }
        setTimeout(pollState, 500);
      })
      .catch((error) => {
        console.error("Polling error:", error);
        setTimeout(pollState, 5000);
      });
  };

  useEffect(() => {
    pollState();
    return () => {
      if (xhrRef.current) {
        xhrRef.current.abort();
      }
    };
  }, []);

  const progressPercentage =
    installerState.totalSteps > 0
      ? (installerState.currentStep / installerState.totalSteps) * 100
      : 0;

  return (
    <div className="flex flex-col gap-4">
      {/* Installer State Section */}
      <div className="mb-4 p-3 bg-gray-100 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 ">
          Installer State
        </h3>
        <div className="flex flex-col gap-2">
          {/* Status Badge */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">
              Status:
            </span>
            <span
              className={`px-2 py-0.5 rounded text-xs font-bold text-white ${STATUS_COLORS[installerState.status]}`}
            >
              {STATUS_LABELS[installerState.status]}
            </span>
          </div>
          {/* Progress Bar */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide w-16">
              Progress:
            </span>
            <span className="text-xs text-gray-700 dark:text-gray-300">
              {installerState.currentStep}/{installerState.totalSteps}
            </span>
            <div className="flex-1 bg-gray-300 dark:bg-gray-700 rounded-full h-2 overflow-hidden">
              <div
                className="h-full bg-cyan-500 transition-all duration-300"
                style={{ width: `${progressPercentage}%` }}
              />
            </div>
          </div>
          {/* Ready States */}
          <div className="grid grid-cols-3 gap-2 text-xs">
            <div className="flex items-center gap-1">
              <span className="text-gray-500 dark:text-gray-400">Conda:</span>
              <span
                className={
                  installerState.condaReady ? "text-green-500" : "text-gray-400"
                }
              >
                {installerState.condaReady ? "✓" : "✗"}
              </span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-gray-500 dark:text-gray-400">Torch:</span>
              <span
                className={
                  installerState.torchReady ? "text-green-500" : "text-gray-400"
                }
              >
                {installerState.torchReady ? "✓" : "✗"}
              </span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-gray-500 dark:text-gray-400">
                React UI:
              </span>
              <span
                className={
                  installerState.reactUIReady
                    ? "text-green-500"
                    : "text-gray-400"
                }
              >
                {installerState.reactUIReady ? "✓" : "✗"}
              </span>
            </div>
          </div>
          {/* Last Error */}
          <div className="flex items-center gap-2 text-xs">
            <span className="text-gray-500 dark:text-gray-400">
              Last Error:
            </span>
            <span className="text-orange-500 dark:text-orange-400 truncate flex-1">
              {installerState.lastError || "None"}
            </span>
          </div>
          {/* Git Hash */}
          <div className="flex items-center gap-2 text-xs">
            <span className="text-gray-500 dark:text-gray-400">Git Hash:</span>
            <span className="text-gray-700 dark:text-gray-300 font-mono">
              {installerState.gitHash || "None"}
            </span>
          </div>
          {/* Timestamp */}
          <div className="flex items-center gap-2 text-xs">
            <span className="text-gray-500 dark:text-gray-400">
              Last Updated:
            </span>
            <span className="text-gray-700 dark:text-gray-300">
              {installerState.timestamp > 0
                ? new Date(installerState.timestamp).toLocaleString()
                : "-"}
            </span>
          </div>
        </div>
      </div>
      {/* Messages Section */}
      <div className="">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">
            Messages
          </span>
        </div>
        <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-2 max-h-32 overflow-auto border border-gray-200 dark:border-gray-700">
          {messages.length > 0 ? (
            messages.map((msg, i) => (
              <div
                key={i}
                className="text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap"
              >
                {msg}
              </div>
            ))
          ) : (
            <span className="text-xs text-gray-400 dark:text-gray-500">
              (no messages)
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
