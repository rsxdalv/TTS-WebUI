// ExtensionsView manages extensions with the app stopped. It calls only
// /api/ext/*, which the launcher serves from the same Go code the app uses.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { client } from "./client";

interface Ext {
  package_name: string;
  name: string;
  description: string;
  website: string;
  extension_class: string;
  kind: "interface" | "decorator" | "host" | "platform";
  proxy: string;
  validation?: string;
  builtin: boolean;
  installed: boolean;
  disabled: boolean;
  can_be_disabled: boolean;
  requires: string[];
  requirements_state: Requirement[];
}

interface Requirement {
  name: string;
  kind: "platform" | "entry";
  state: "ready" | "missing";
  via?: string;
  would?: string;
  installable?: boolean;
}

// GPUAsk is the answer to an install that needs the uv platform before the user
// has chosen the GPU (409, needs: gpu_choice). Nothing was installed.
interface GPUAsk {
  proposed: string;
  reason: string;
  choices: { label: string; row: string; approx_mb: number }[];
}

// blocker is the first requirement that installing this entry cannot satisfy:
// a platform (or the Gradio host) that is not there and cannot be installed
// from here. A platform that can be installed (uv) is installed first.
function blocker(e: Ext): Requirement | undefined {
  return e.requirements_state.find(
    (r) =>
      r.state === "missing" && !r.installable && !(r.would ?? "").startsWith("would install"),
  );
}

// installsHost is whether installing this entry also installs the Gradio host:
// the host itself, or a tab that runs inside it while it is not installed.
function installsHost(e: Ext): boolean {
  return (
    e.kind === "host" ||
    e.requirements_state.some((r) => r.name === "gradio-host" && r.state === "missing")
  );
}

function requirementText(r: Requirement): string {
  if (r.state === "ready") return `${r.name} (${r.via})`;
  return `${r.name}: ${r.would ?? "missing"}`;
}

interface ListAnswer {
  extensions: Ext[];
  app_running: boolean;
  warning?: string;
  platform: string;
  platform_ready: boolean;
  platform_detail?: string;
  requires_error?: string;
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
  gpu?: string,
  signal?: AbortSignal,
  audioRow?: string,
): Promise<boolean | GPUAsk> {
  const r = await client(`/api/ext/op/${op}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      package_name: packageName,
      ...(gpu ? { gpu } : {}),
      ...(audioRow ? { audiocpp_row: audioRow } : {}),
    }),
    signal,
  });
  if (op === "latest-version") {
    const body = await r.json();
    onLine(typeof body === "string" ? body : (body.error ?? JSON.stringify(body)));
    return r.ok;
  }
  if (!r.ok || !r.body) {
    const body = await r.json().catch(() => ({}));
    if (r.status === 409 && body.needs === "gpu_choice") return body as GPUAsk;
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

  const [ask, setAsk] = useState<{ ext: Ext; ask: GPUAsk; pick: string } | null>(null);
  const abort = useRef<AbortController | null>(null);
  // The audio.cpp build (cpu or vulkan) is chosen for an install that installs the
  // audiocpp platform: the platform itself, or a model while it is missing.
  const [audioRow, setAudioRow] = useState("cpu");
  const installsAudio = (e: Ext) =>
    e.package_name === "audiocpp" ||
    e.requirements_state.some((r) => r.name === "audiocpp" && r.state === "missing");

  const run = async (ext: Ext, op: Op, gpu?: string) => {
    setBusy(ext.package_name);
    setAsk(null);
    const row = op === "install" && installsAudio(ext) ? audioRow : undefined;
    setLog([
      `> ${op} ${ext.package_name}${gpu ? ` (GPU: ${gpu})` : ""}${row ? ` (audio.cpp build: ${row})` : ""}`,
    ]);
    abort.current = new AbortController();
    try {
      const res = await runOp(
        op,
        ext.package_name,
        (l) => setLog((prev) => [...prev, l]),
        gpu,
        abort.current.signal,
        row,
      );
      if (typeof res === "object") {
        // The uv platform is installed first and needs the GPU choice.
        setLog((prev) => [...prev, "The uv platform must be installed first: choose the GPU."]);
        setAsk({ ext, ask: res, pick: res.proposed });
        return;
      }
      if (op === "install" || op === "uninstall") setChanged(true);
      setLog((prev) => [...prev, res ? "Done." : "Failed."]);
    } catch (e: unknown) {
      const cancelled = e instanceof DOMException && e.name === "AbortError";
      setLog((prev) => [
        ...prev,
        cancelled
          ? "Cancelled. Install again to continue; downloads are kept."
          : `Error: ${e instanceof Error ? e.message : String(e)}`,
      ]);
    } finally {
      abort.current = null;
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
      const key =
        e.kind === "host"
          ? "Gradio host"
          : e.kind === "platform"
            ? "Platforms"
            : e.kind === "decorator"
              ? "Decorators"
              : e.extension_class === "audiocpp"
                ? "audio.cpp models"
                : title(e.extension_class || "other");
      out.set(key, [...(out.get(key) ?? []), e]);
    }
    for (const list of out.values()) {
      list.sort(
        (a, b) => Number(b.installed) - Number(a.installed) || a.name.localeCompare(b.name),
      );
    }
    const rank = (g: string) => (g === "Gradio host" || g === "Platforms" ? 0 : g === "Decorators" ? 2 : 1);
    return [...out.entries()].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b));
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
      <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-300 text-sm">
        Runs on: <strong>{data.platform}</strong>
        {data.platform_ready ? "" : ` — not available: ${data.platform_detail ?? ""}`}
      </div>
      {data.requires_error && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
          {data.requires_error}
        </div>
      )}
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
                      {e.validation === "unvalidated" ? " · unvalidated (installs and starts; no audio checked)" : ""}
                    </span>
                  </div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">{e.description}</p>
                  {e.requires.length > 0 && !e.builtin && (
                    <p className="text-xs text-gray-600 dark:text-gray-400">
                      Requires: {e.requirements_state.map(requirementText).join(", ")}
                    </p>
                  )}
                  {!e.installed && installsAudio(e) && (
                    <div className="mt-1 mb-1 flex items-center gap-2 text-xs">
                      <label htmlFor={`audio-row-${e.package_name}`}>audio.cpp build to install:</label>
                      <select
                        id={`audio-row-${e.package_name}`}
                        className="px-2 py-1 rounded border border-gray-300 dark:border-gray-700 bg-transparent"
                        value={audioRow}
                        onChange={(ev) => setAudioRow(ev.target.value)}
                      >
                        <option value="cpu">CPU (34 MB, runs anywhere)</option>
                        <option value="vulkan">Vulkan (84 MB, needs a GPU driver with Vulkan)</option>
                      </select>
                    </div>
                  )}
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
                    <button
                      className={btnPrimary}
                      disabled={!!busy || !!blocker(e)}
                      title={blocker(e) ? requirementText(blocker(e)!) : undefined}
                      onClick={() => run(e, "install")}
                    >
                      Install
                    </button>
                  )}
                  {!e.builtin && e.installed && (
                    <>
                      {e.kind !== "host" && e.kind !== "platform" && (
                        <>
                          <button className={btn} disabled={!!busy} onClick={() => run(e, "latest-version")}>
                            Check update
                          </button>
                          <button className={btn} disabled={!!busy} onClick={() => run(e, "install")}>
                            Update
                          </button>
                        </>
                      )}
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
      {(ask || busy || log.length > 0) && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          role="dialog"
          aria-modal="true"
          aria-label={ask ? "Choose the GPU" : "Install log"}
        >
          <div className="w-full max-w-2xl max-h-[90vh] overflow-auto rounded-xl border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 p-4 shadow-xl flex flex-col gap-3">
      {ask && (
        <div className="text-sm flex flex-col gap-2">
          <strong>Which GPU is this for?</strong>
          <p className="text-gray-700 dark:text-gray-300">
            {installsHost(ask.ext)
              ? `${ask.ext.name} needs the Gradio host, which is installed first on the uv platform: uv, Python, torch for the GPU you choose and the app's requirements.`
              : `${ask.ext.name} runs on the uv platform, which is installed first: uv, Python and torch for the GPU you choose.`}{" "}
            Proposed: <strong>{ask.ask.proposed}</strong> ({ask.ask.reason}).
            Every engine on the same torch build shares its files; another build is a separate
            download.
          </p>
          <select
            className="px-2 py-1 rounded border border-gray-300 dark:border-gray-700 bg-transparent"
            value={ask.pick}
            onChange={(e) => setAsk({ ...ask, pick: e.target.value })}
          >
            {ask.ask.choices.map((c) => (
              <option key={c.label} value={c.label}>
                {c.label} (torch {c.row}, about {(c.approx_mb / 1000).toFixed(1)} GB)
              </option>
            ))}
          </select>
          <div className="flex gap-2">
            <button className={btnPrimary} onClick={() => run(ask.ext, "install", ask.pick)}>
              {installsHost(ask.ext)
                ? ask.ext.kind === "host"
                  ? "Install uv and the Gradio host"
                  : `Install uv, the Gradio host and ${ask.ext.name}`
                : `Install uv and ${ask.ext.name}`}
            </button>
            <button className={btn} onClick={() => {
                setAsk(null);
                setLog([]);
              }}>
              Not now
            </button>
          </div>
        </div>
      )}
      {busy && (
        <div>
          <button className={btn} onClick={() => abort.current?.abort()}>
            Cancel
          </button>
        </div>
      )}
      {log.length > 0 && (
        <pre className="p-3 rounded-lg bg-gray-900 text-gray-100 text-xs overflow-auto max-h-72 whitespace-pre-wrap">
          {log.join("\n")}
        </pre>
      )}
            {!busy && !ask && (
              <div>
                <button className={btn} onClick={() => setLog([])}>
                  Close
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
