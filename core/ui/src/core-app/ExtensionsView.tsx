// ExtensionsView manages extensions with the app stopped. It calls only
// /api/ext/*, which the launcher serves from the same Go code the app uses.
import { useCallback, useEffect, useMemo, useState } from "react";
import { client } from "./client";

interface Ext {
  package_name: string;
  name: string;
  description: string;
  website: string;
  extension_class: string;
  kind: "interface" | "decorator";
  proxy: string;
  builtin: boolean;
  installed: boolean;
  disabled: boolean;
  can_be_disabled: boolean;
}

interface ListAnswer {
  extensions: Ext[];
  app_running: boolean;
  warning?: string;
}

type Op = "install" | "uninstall" | "check-dependencies" | "latest-version";

const btn =
  "px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed";
const btnPrimary =
  "px-3 py-1 text-sm rounded bg-sky-600 text-white hover:bg-sky-700 disabled:opacity-50 disabled:cursor-not-allowed";

function title(s: string) {
  return s.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// runOp streams an install-style operation. Output arrives as NDJSON:
// {"line": "..."} per line, then {"done": true, "ok": bool}.
async function runOp(
  op: Op,
  packageName: string,
  onLine: (line: string) => void,
): Promise<boolean> {
  const r = await client(`/api/ext/op/${op}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ package_name: packageName }),
  });
  if (op === "latest-version") {
    const body = await r.json();
    onLine(typeof body === "string" ? body : (body.error ?? JSON.stringify(body)));
    return r.ok;
  }
  if (!r.ok || !r.body) {
    const body = await r.json().catch(() => ({}));
    onLine(`Error: ${body.error ?? `HTTP ${r.status}`}`);
    return false;
  }
  const reader = r.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let ok = false;
  const handle = (text: string) => {
    if (!text.trim()) return;
    const msg = JSON.parse(text);
    if (typeof msg.line === "string") onLine(msg.line);
    if (msg.done) ok = !!msg.ok;
  };
  for (;;) {
    const { value, done } = await reader.read();
    if (value) buf += decoder.decode(value, { stream: !done });
    const parts = buf.split("\n");
    buf = parts.pop() ?? "";
    parts.forEach(handle);
    if (done) break;
  }
  handle(buf);
  return ok;
}

export function ExtensionsView() {
  const [data, setData] = useState<ListAnswer | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [log, setLog] = useState<string[]>([]);
  const [changed, setChanged] = useState(false);

  const load = useCallback(async () => {
    try {
      const r = await client("/api/ext/list");
      const body = await r.json();
      if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`);
      setData(body);
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (ext: Ext, op: Op) => {
    setBusy(ext.package_name);
    setLog([`> ${op} ${ext.package_name}`]);
    try {
      const ok = await runOp(op, ext.package_name, (l) =>
        setLog((prev) => [...prev, l]),
      );
      if (op === "install" || op === "uninstall") setChanged(true);
      setLog((prev) => [...prev, ok ? "Done." : "Failed."]);
    } catch (e: unknown) {
      setLog((prev) => [...prev, `Error: ${e instanceof Error ? e.message : String(e)}`]);
    } finally {
      setBusy(null);
      await load();
    }
  };

  const toggle = async (ext: Ext) => {
    setBusy(ext.package_name);
    try {
      const r = await client("/api/ext/enabled", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ package_name: ext.package_name, enabled: ext.disabled }),
      });
      const body = await r.json();
      if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`);
      setChanged(true);
      setLog([`${ext.disabled ? "Enabled" : "Disabled"} ${ext.package_name}`]);
    } catch (e: unknown) {
      setLog([`Error: ${e instanceof Error ? e.message : String(e)}`]);
    } finally {
      setBusy(null);
      await load();
    }
  };

  const groups = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const out = new Map<string, Ext[]>();
    for (const e of data?.extensions ?? []) {
      if (q && !`${e.name} ${e.package_name} ${e.description}`.toLowerCase().includes(q)) continue;
      const key = e.kind === "decorator" ? "Decorators" : title(e.extension_class || "other");
      out.set(key, [...(out.get(key) ?? []), e]);
    }
    for (const list of out.values()) {
      list.sort(
        (a, b) => Number(b.installed) - Number(a.installed) || a.name.localeCompare(b.name),
      );
    }
    return [...out.entries()].sort(([a], [b]) =>
      a === "Decorators" ? 1 : b === "Decorators" ? -1 : a.localeCompare(b),
    );
  }, [data, filter]);

  if (error) {
    return (
      <div className="p-4 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
        {error}
      </div>
    );
  }
  if (!data) return <p className="text-gray-500 dark:text-gray-400 text-sm">Reading extensions…</p>;

  return (
    <div className="flex flex-col gap-4">
      {data.warning && (
        <div className="p-3 rounded-lg bg-amber-50 dark:bg-amber-950 text-amber-800 dark:text-amber-200 text-sm">
          {data.warning}
        </div>
      )}
      {(changed || data.app_running) && (
        <div className="p-3 rounded-lg bg-sky-50 dark:bg-sky-950 text-sky-800 dark:text-sky-200 text-sm">
          {data.app_running
            ? "The app is running: changes apply after you restart it."
            : "Changes apply the next time the app starts."}
        </div>
      )}
      <input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="Filter extensions"
        className="px-3 py-2 text-sm rounded border border-gray-300 dark:border-gray-700 bg-transparent"
      />
      {groups.map(([group, list]) => (
        <section key={group} className="flex flex-col gap-2">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
            {group}
          </h2>
          {list.map((e) => (
            <div
              key={e.package_name}
              className="p-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="font-medium">
                    {e.name}{" "}
                    <span className="text-xs font-normal text-gray-500 dark:text-gray-400">
                      {e.builtin
                        ? "built-in"
                        : e.installed
                          ? e.disabled
                            ? "installed, disabled"
                            : "installed"
                          : "not installed"}
                      {e.proxy === "native" ? " · own venv" : ""}
                    </span>
                  </div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">{e.description}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-500">
                    {e.package_name}
                    {e.website && (
                      <>
                        {" · "}
                        <a className="underline" href={e.website} target="_blank" rel="noreferrer">
                          website
                        </a>
                      </>
                    )}
                  </p>
                </div>
                <div className="flex flex-wrap gap-2 justify-end shrink-0">
                  {!e.builtin && !e.installed && (
                    <button className={btnPrimary} disabled={!!busy} onClick={() => run(e, "install")}>
                      Install
                    </button>
                  )}
                  {!e.builtin && e.installed && (
                    <>
                      <button className={btn} disabled={!!busy} onClick={() => run(e, "latest-version")}>
                        Check update
                      </button>
                      <button className={btn} disabled={!!busy} onClick={() => run(e, "install")}>
                        Update
                      </button>
                      <button className={btn} disabled={!!busy} onClick={() => run(e, "uninstall")}>
                        Uninstall
                      </button>
                    </>
                  )}
                  {e.can_be_disabled && e.installed && (
                    <button className={btn} disabled={!!busy} onClick={() => toggle(e)}>
                      {e.disabled ? "Enable" : "Disable"}
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </section>
      ))}
      {log.length > 0 && (
        <pre className="p-3 rounded-lg bg-gray-900 text-gray-100 text-xs overflow-auto max-h-72 whitespace-pre-wrap">
          {log.join("\n")}
        </pre>
      )}
    </div>
  );
}
