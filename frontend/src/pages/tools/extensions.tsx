import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/dialog";
import { LauncherExtensionsLink } from "../../components/LauncherExtensionsLink";
import { errorText, goGetJson } from "../../functions/goData";

type Entry = {
  package_name?: string;
  name?: string;
  description?: string;
  extension_class?: string;
  author?: string;
  website?: string;
  requirements?: string;
  proxy?: string;
};

/** What the control API says about an entry (installed, version, enabled). */
type State = { installed: boolean; version?: string; enabled: boolean; proxy?: string };

type Kind = "interface" | "decorator";
type Op = "install" | "uninstall";

type Run = {
  op: Op;
  name: string;
  proxy: string;
  lines: string[];
  step: string;
  done: boolean;
  ok: boolean;
  needsGpu: boolean;
};

/** Built-in tabs are part of the host (as in interface_extensions._is_builtin_tab): no install or uninstall. */
const isBuiltin = (e: Entry) =>
  e.requirements === "builtin" ||
  ((e.package_name ?? "").includes("builtin") && e.proxy !== "native");

/**
 * Streams one install or uninstall (the control API answers NDJSON: {line},
 * {plan}, {step}, then {done, ok}). Only a package_name is sent; the Go side
 * looks it up in the catalog.
 */
async function runOp(
  op: Op,
  packageName: string,
  onUpdate: (patch: (r: Run) => Run) => void
): Promise<void> {
  const response = await fetch(`/api/go-data/extensions/${op}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ package_name: packageName }),
  });
  if (!response.ok || !response.body) {
    const body = await response
      .json()
      .catch(() => ({} as { error?: string; needs?: string }));
    onUpdate((r) => ({
      ...r,
      lines: [...r.lines, `Error: ${body.error ?? `HTTP ${response.status}`}`],
      done: true,
      ok: false,
      needsGpu: response.status === 409 && body.needs === "gpu_choice",
    }));
    return;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const handle = (text: string) => {
    if (!text.trim()) return;
    const msg = JSON.parse(text);
    if (typeof msg.line === "string") {
      onUpdate((r) => ({ ...r, lines: [...r.lines, msg.line] }));
    }
    if (msg.step?.id) {
      onUpdate((r) => ({ ...r, step: `${msg.step.id}: ${msg.step.state}` }));
    }
    if (msg.done) onUpdate((r) => ({ ...r, done: true, ok: !!msg.ok }));
  };
  for (;;) {
    const { value, done } = await reader.read();
    if (value) buffer += decoder.decode(value, { stream: !done });
    const parts = buffer.split("\n");
    buffer = parts.pop() ?? "";
    parts.forEach(handle);
    if (done) {
      handle(buffer);
      break;
    }
  }
  onUpdate((r) =>
    r.done
      ? r
      : { ...r, done: true, ok: false, lines: [...r.lines, "The answer ended early."] }
  );
}

/**
 * The extension lists (interface tabs and decorators) from the Go catalog, with
 * install and uninstall. The same operations the launcher's Extensions page
 * runs; the Gradio list tabs are gone.
 */
const ExtensionsPage = () => {
  const [kind, setKind] = React.useState<Kind>("interface");
  const [entries, setEntries] = React.useState<Entry[] | null>(null);
  const [states, setStates] = React.useState<Record<string, State>>({});
  const [filter, setFilter] = React.useState("");
  const [error, setError] = React.useState("");
  const [run, setRun] = React.useState<Run | null>(null);
  const logRef = React.useRef<HTMLPreElement>(null);
  // The dialog keeps showing the last run while it fades out.
  const lastRun = React.useRef<Run | null>(null);
  if (run) lastRun.current = run;
  const view = run ?? lastRun.current;

  const loadStates = React.useCallback(
    () =>
      goGetJson<Record<string, State>>(`/api/go-data/extensions/${kind}/state`)
        .then(setStates)
        .catch(() => undefined),
    [kind]
  );

  React.useEffect(() => {
    setEntries(null);
    setStates({});
    setError("");
    goGetJson<Entry[]>(`/api/go-data/extensions/${kind}`)
      .then(setEntries)
      .catch((e) => setError(errorText(e)));
    loadStates();
  }, [kind, loadStates]);

  React.useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [run?.lines.length]);

  const start = async (e: Entry, op: Op) => {
    const name = e.name || e.package_name || "";
    if (op === "uninstall" && !window.confirm(`Uninstall ${name}?`)) return;
    setRun({
      op,
      name,
      proxy: e.proxy ?? "",
      lines: [],
      step: "",
      done: false,
      ok: false,
      needsGpu: false,
    });
    try {
      await runOp(op, e.package_name ?? "", (patch) =>
        setRun((r) => (r ? patch(r) : r))
      );
    } catch (err) {
      setRun((r) =>
        r
          ? { ...r, lines: [...r.lines, `Error: ${errorText(err)}`], done: true, ok: false }
          : r
      );
    }
    loadStates();
  };

  const f = filter.trim().toLowerCase();
  const shown = (entries ?? []).filter((e) =>
    [e.name, e.package_name, e.description, e.extension_class]
      .join(" ")
      .toLowerCase()
      .includes(f)
  );
  const busy = !!run && !run.done;

  const statusOf = (e: Entry) => {
    if (isBuiltin(e)) return "built in";
    const s = states[e.package_name ?? ""];
    if (!s) return "";
    const base = s.installed
      ? `installed${s.version ? ` ${s.version}` : ""}`
      : "not installed";
    return s.enabled ? base : `${base}, disabled`;
  };

  return (
    <Template title="Extensions">
      <div className="flex w-full flex-col gap-3 p-4">
        <div className="flex gap-2 items-center">
          <Button
            variant={kind === "interface" ? "default" : "outline"}
            onClick={() => setKind("interface")}
          >
            Extensions
          </Button>
          <Button
            variant={kind === "decorator" ? "default" : "outline"}
            onClick={() => setKind("decorator")}
          >
            Decorators
          </Button>
          <input
            className="border rounded px-2 py-1 flex-1"
            placeholder="Filter"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        {error && <div className="text-red-600">{error}</div>}
        {entries && (
          <table className="text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-1 pr-4">Name</th>
                <th className="py-1 pr-4">Class</th>
                <th className="py-1 pr-4">Status</th>
                <th className="py-1 pr-4">Description</th>
                <th className="py-1" />
              </tr>
            </thead>
            <tbody>
              {shown.map((e) => {
                const s = states[e.package_name ?? ""];
                return (
                  <tr key={e.package_name} className="border-b border-gray-100 align-top">
                    <td className="py-1 pr-4">
                      {e.website ? (
                        <a className="underline" href={e.website} target="_blank" rel="noreferrer">
                          {e.name}
                        </a>
                      ) : (
                        e.name
                      )}
                      <div className="text-xs text-gray-500">
                        {e.package_name}
                        {e.proxy ? ` (${e.proxy})` : ""}
                      </div>
                    </td>
                    <td className="py-1 pr-4">{e.extension_class}</td>
                    <td className="py-1 pr-4 whitespace-nowrap">{statusOf(e)}</td>
                    <td className="py-1 pr-4">{e.description}</td>
                    <td className="py-1 text-right whitespace-nowrap">
                      {!isBuiltin(e) && s && (
                        <Button
                          size="sm"
                          variant={s.installed ? "outline" : "default"}
                          disabled={busy}
                          onClick={() => start(e, s.installed ? "uninstall" : "install")}
                        >
                          {s.installed ? "Uninstall" : "Install"}
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      <Dialog open={!!run} onOpenChange={(open) => !open && run?.done && setRun(null)}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>
              {view?.op === "install" ? "Installing" : "Uninstalling"} {view?.name}
            </DialogTitle>
            <DialogDescription>
              {!view?.done
                ? view?.step || "Working. Installs can take several minutes."
                : view?.ok
                  ? view?.proxy === "native"
                    ? "Done."
                    : "Done. Restart the app to load or unload it."
                  : "It did not finish."}
            </DialogDescription>
          </DialogHeader>
          <pre
            ref={logRef}
            className="max-h-[50vh] overflow-auto rounded bg-gray-100 p-2 text-xs dark:bg-gray-900"
          >
            {view?.lines.join("\n")}
          </pre>
          {view?.needsGpu && (
            <p className="text-sm">
              This install needs a GPU choice first. Make it on the{" "}
              <LauncherExtensionsLink text="Extensions page of the launcher" />.
            </p>
          )}
          <DialogFooter>
            <Button disabled={!view?.done} onClick={() => setRun(null)}>
              {view?.done ? "Close" : "Working…"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Template>
  );
};

export default ExtensionsPage;
