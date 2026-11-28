import React from "react";
import { Button } from "./ui/button";

/**
 * The management panel of an extension that runs in its own venv: the buttons
 * of the Gradio tab's "Manage <name> Extension" box (Install / Update,
 * Uninstall, Disable / Enable, Start with WebUI) and its Start / Shutdown row,
 * with a console under them.
 *
 * The panel only holds the buttons for now: each action is an optional
 * handler, and one that is not given writes "not connected" to the console.
 * Wiring an action means passing its handler (it may append to the console
 * through the `log` it is given).
 */
export type ExtensionActions = {
  install?: (log: (line: string) => void) => Promise<void> | void;
  uninstall?: (log: (line: string) => void) => Promise<void> | void;
  start?: (log: (line: string) => void) => Promise<void> | void;
  shutdown?: (log: (line: string) => void) => Promise<void> | void;
  setEnabled?: (enabled: boolean, log: (line: string) => void) => Promise<void> | void;
  setAutostart?: (autostart: boolean, log: (line: string) => void) => Promise<void> | void;
};

export const ExtensionManager = ({
  name,
  installed,
  running,
  enabled = true,
  autostart = false,
  actions = {},
  defaultOpen = true,
}: {
  name: string;
  installed: boolean;
  running: boolean;
  enabled?: boolean;
  autostart?: boolean;
  actions?: ExtensionActions;
  defaultOpen?: boolean;
}) => {
  const [open, setOpen] = React.useState(defaultOpen);
  const [busy, setBusy] = React.useState("");
  const [lines, setLines] = React.useState<string[]>([]);
  const [enabledNow, setEnabledNow] = React.useState(enabled);
  const [autostartNow, setAutostartNow] = React.useState(autostart);
  const consoleRef = React.useRef<HTMLPreElement>(null);

  React.useEffect(() => setEnabledNow(enabled), [enabled]);
  React.useEffect(() => setAutostartNow(autostart), [autostart]);
  React.useEffect(() => {
    consoleRef.current?.scrollTo({ top: consoleRef.current.scrollHeight });
  }, [lines]);

  const log = (line: string) => setLines((l) => [...l, line]);

  type Handler = ((log: (line: string) => void) => Promise<void> | void) | undefined;
  const run = async (label: string, handler: Handler) => {
    setBusy(label);
    try {
      if (!handler) log(`${label}: not connected yet`);
      else await handler(log);
    } catch (e) {
      log(`${label} failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setBusy("");
    }
  };

  const disabled = !!busy;

  return (
    <div className="w-full rounded-md border bg-card text-card-foreground p-3 flex flex-col gap-3">
      <button
        type="button"
        className="flex w-full items-center justify-between text-left font-medium"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <span>Manage {name} Extension</span>
        <span aria-hidden>{open ? "▼" : "◀"}</span>
      </button>

      {open && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              disabled={disabled}
              onClick={() => run(installed ? "Update" : "Install", actions.install)}
            >
              {busy === "Install" || busy === "Update"
                ? "Working…"
                : installed
                  ? "Update (uv venv)"
                  : "Install (uv venv)"}
            </Button>
            <label className="flex h-10 items-center gap-2 rounded-md border px-3 text-sm">
              <input
                type="checkbox"
                checked={autostartNow}
                disabled={disabled}
                onChange={(e) => {
                  const v = e.target.checked;
                  setAutostartNow(v);
                  run("Start with WebUI", actions.setAutostart && ((l) => actions.setAutostart!(v, l)));
                }}
              />
              Start with WebUI
            </label>
            <Button
              variant="secondary"
              disabled={disabled}
              onClick={() => {
                const v = !enabledNow;
                setEnabledNow(v);
                run(v ? "Enable" : "Disable", actions.setEnabled && ((l) => actions.setEnabled!(v, l)));
              }}
            >
              {enabledNow ? "Disable" : "Enable"}
            </Button>
            <Button
              variant="destructive"
              disabled={disabled || !installed}
              onClick={() => run("Uninstall", actions.uninstall)}
            >
              Uninstall
            </Button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              disabled={disabled || !running}
              onClick={() => run("Shutdown", actions.shutdown)}
            >
              Shutdown Extension
            </Button>
            <Button
              disabled={disabled || running || !installed}
              onClick={() => run("Start", actions.start)}
            >
              Start Extension
            </Button>
            <span className="text-sm text-muted-foreground">
              {running ? "Running" : installed ? "Stopped" : "Not installed"}
            </span>
          </div>

          <div>
            <div className="mb-1 text-sm font-medium">Console</div>
            <pre
              ref={consoleRef}
              className="h-32 overflow-auto rounded-md border bg-muted text-foreground p-2 text-xs"
            >
              {lines.join("\n")}
            </pre>
          </div>
        </>
      )}
    </div>
  );
};
