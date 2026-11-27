// ExtensionsView manages extensions with the app stopped. It calls only
// /api/ext/*, which the launcher serves from the same Go code the app uses.
import {
  createMemo,
  createSignal,
  createStore,
  Errored,
  For,
  Loading,
  refresh,
  Show,
} from "solid-js";
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

// The state of the "which GPU is this for?" prompt, once an install says it needs one.
interface AskState {
  ext: Ext;
  ask: GPUAsk;
  pick: string;
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

// The list is a read: a failure throws, and the <Errored> boundary shows it.
async function fetchList(): Promise<ListAnswer> {
  const r = await client("/api/ext/list");
  const body = await r.json();
  if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`);
  return body;
}

const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e));

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
  // One GET on mount, and again after every operation. A failure throws to the
  // <Errored> boundary below, which replaces the view until a read succeeds.
  const list = createMemo(fetchList);
  const [filter, setFilter] = createSignal("");
  const [busy, setBusy] = createSignal<string | null>(null);
  const [log, setLog] = createStore<string[]>([]);
  const [changed, setChanged] = createSignal(false);
  const [ask, setAsk] = createSignal<AskState | null>(null);
  let abort: AbortController | null = null;

  const addLog = (line: string) =>
    setLog((lines) => {
      lines.push(line);
    });
  const resetLog = (...lines: string[]) => setLog(() => lines);

  // The audio.cpp build (cpu or vulkan) is chosen for an install that installs the
  // audiocpp platform: the platform itself, or a model while it is missing.
  const [audioRow, setAudioRow] = createSignal("cpu");
  const installsAudio = (e: Ext) =>
    e.package_name === "audiocpp" ||
    e.requirements_state.some((r) => r.name === "audiocpp" && r.state === "missing");

  // Operations stream their output into the log while they run, so they are
  // plain async handlers: inside an action, writes would only land at the end.
  const run = async (ext: Ext, op: Op, gpu?: string) => {
    setBusy(ext.package_name);
    setAsk(null);
    const row = op === "install" && installsAudio(ext) ? audioRow() : undefined;
    resetLog(
      `> ${op} ${ext.package_name}${gpu ? ` (GPU: ${gpu})` : ""}${row ? ` (audio.cpp build: ${row})` : ""}`,
    );
    abort = new AbortController();
    try {
      const res = await runOp(op, ext.package_name, addLog, gpu, abort.signal, row);
      if (typeof res === "object") {
        // The uv platform is installed first and needs the GPU choice.
        addLog("The uv platform must be installed first: choose the GPU.");
        setAsk({ ext, ask: res, pick: res.proposed });
        return;
      }
      if (op === "install" || op === "uninstall") setChanged(true);
      addLog(res ? "Done." : "Failed.");
    } catch (e: unknown) {
      const cancelled = e instanceof DOMException && e.name === "AbortError";
      addLog(
        cancelled
          ? "Cancelled. Install again to continue; downloads are kept."
          : `Error: ${errorText(e)}`,
      );
    } finally {
      abort = null;
      setBusy(null);
      refresh(list);
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
      resetLog(`${ext.disabled ? "Enabled" : "Disabled"} ${ext.package_name}`);
    } catch (e: unknown) {
      resetLog(`Error: ${errorText(e)}`);
    } finally {
      setBusy(null);
      refresh(list);
    }
  };

  const groups = createMemo(() => {
    const q = filter().trim().toLowerCase();
    const out = new Map<string, Ext[]>();
    for (const e of list().extensions) {
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
    for (const exts of out.values()) {
      exts.sort(
        (a, b) => Number(b.installed) - Number(a.installed) || a.name.localeCompare(b.name),
      );
    }
    const rank = (g: string) => (g === "Gradio host" || g === "Platforms" ? 0 : g === "Decorators" ? 2 : 1);
    return [...out.entries()].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b));
  });

  return (
    <Errored
      fallback={(err) => (
        <div class="p-4 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
          {errorText(err())}
        </div>
      )}
    >
      <Loading
        fallback={<p class="text-gray-500 dark:text-gray-400 text-sm">Reading extensions…</p>}
      >
        <div class="flex flex-col gap-4">
          <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-300 text-sm">
            Runs on: <strong>{list().platform}</strong>
            {list().platform_ready ? "" : ` — not available: ${list().platform_detail ?? ""}`}
          </div>
          <Show when={list().requires_error}>
            {(message) => (
              <div class="p-3 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
                {message()}
              </div>
            )}
          </Show>
          <Show when={list().warning}>
            {(message) => (
              <div class="p-3 rounded-lg bg-amber-50 dark:bg-amber-950 text-amber-800 dark:text-amber-200 text-sm">
                {message()}
              </div>
            )}
          </Show>
          <Show when={changed() || list().app_running}>
            <div class="p-3 rounded-lg bg-sky-50 dark:bg-sky-950 text-sky-800 dark:text-sky-200 text-sm">
              {list().app_running
                ? "The app is running: changes apply after you restart it."
                : "Changes apply the next time the app starts."}
            </div>
          </Show>
          <input
            value={filter()}
            onInput={(e) => setFilter(e.currentTarget.value)}
            placeholder="Filter extensions"
            class="px-3 py-2 text-sm rounded border border-gray-300 dark:border-gray-700 bg-transparent"
          />
          <Show when={list().extensions.some((e) => e.package_name === "audiocpp" && !e.installed)}>
            <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-900 text-sm flex items-center gap-2">
              <label for="audio-row">audio.cpp build to install:</label>
              <select
                id="audio-row"
                class="px-2 py-1 rounded border border-gray-300 dark:border-gray-700 bg-transparent"
                onChange={(e) => setAudioRow(e.currentTarget.value)}
              >
                <option value="cpu" selected={audioRow() === "cpu"}>
                  CPU (34 MB, runs anywhere)
                </option>
                <option value="vulkan" selected={audioRow() === "vulkan"}>
                  Vulkan (84 MB, needs a GPU driver with Vulkan)
                </option>
              </select>
            </div>
          </Show>
          <For each={groups()} keyed={([group]) => group}>
            {(entry) => (
              <section class="flex flex-col gap-2">
                <h2 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {entry()[0]}
                </h2>
                <For each={entry()[1]} keyed={(e) => e.package_name}>
                  {(e) => (
                    <div class="p-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <div class="font-medium">
                            {e().name}{" "}
                            <span class="text-xs font-normal text-gray-500 dark:text-gray-400">
                              {e().builtin
                                ? "built-in"
                                : e().installed
                                  ? e().disabled
                                    ? "installed, disabled"
                                    : "installed"
                                  : "not installed"}
                              {e().proxy === "native" ? " · own venv" : ""}
                              {e().validation === "unvalidated"
                                ? " · unvalidated (installs and starts; no audio checked)"
                                : ""}
                            </span>
                          </div>
                          <p class="text-sm text-gray-600 dark:text-gray-400">{e().description}</p>
                          <Show when={e().requires.length > 0 && !e().builtin}>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                              Requires: {e().requirements_state.map(requirementText).join(", ")}
                            </p>
                          </Show>
                          <p class="text-xs text-gray-500 dark:text-gray-500">
                            {e().package_name}
                            <Show when={e().website}>
                              {(website) => (
                                <>
                                  {" · "}
                                  <a class="underline" href={website()} target="_blank" rel="noreferrer">
                                    website
                                  </a>
                                </>
                              )}
                            </Show>
                          </p>
                        </div>
                        <div class="flex flex-wrap gap-2 justify-end shrink-0">
                          <Show when={!e().builtin && !e().installed}>
                            <button
                              class={btnPrimary}
                              disabled={!!busy() || !!blocker(e())}
                              title={blocker(e()) ? requirementText(blocker(e())!) : undefined}
                              onClick={() => void run(e(), "install")}
                            >
                              Install
                            </button>
                          </Show>
                          <Show when={!e().builtin && e().installed}>
                            <Show when={e().kind !== "host" && e().kind !== "platform"}>
                              <button
                                class={btn}
                                disabled={!!busy()}
                                onClick={() => void run(e(), "latest-version")}
                              >
                                Check update
                              </button>
                              <button class={btn} disabled={!!busy()} onClick={() => void run(e(), "install")}>
                                Update
                              </button>
                            </Show>
                            <button class={btn} disabled={!!busy()} onClick={() => void run(e(), "uninstall")}>
                              Uninstall
                            </button>
                          </Show>
                          <Show when={e().can_be_disabled && e().installed}>
                            <button class={btn} disabled={!!busy()} onClick={() => void toggle(e())}>
                              {e().disabled ? "Enable" : "Disable"}
                            </button>
                          </Show>
                        </div>
                      </div>
                    </div>
                  )}
                </For>
              </section>
            )}
          </For>
          <Show when={ask()}>
            {(a) => (
              <div class="p-3 rounded-lg border border-sky-300 dark:border-sky-800 bg-sky-50 dark:bg-sky-950 text-sm flex flex-col gap-2">
                <strong>Which GPU is this for?</strong>
                <p class="text-gray-700 dark:text-gray-300">
                  {installsHost(a().ext)
                    ? `${a().ext.name} needs the Gradio host, which is installed first on the uv platform: uv, Python, torch for the GPU you choose and the app's requirements.`
                    : `${a().ext.name} runs on the uv platform, which is installed first: uv, Python and torch for the GPU you choose.`}{" "}
                  Proposed: <strong>{a().ask.proposed}</strong> ({a().ask.reason}).
                  Every engine on the same torch build shares its files; another build is a separate
                  download.
                </p>
                <select
                  class="px-2 py-1 rounded border border-gray-300 dark:border-gray-700 bg-transparent"
                  onChange={(e) => {
                    const pick = e.currentTarget.value;
                    setAsk((prev) => prev && { ...prev, pick });
                  }}
                >
                  <For each={a().ask.choices} keyed={(c) => c.label}>
                    {(c) => (
                      <option value={c().label} selected={c().label === a().pick}>
                        {c().label} (torch {c().row}, about {(c().approx_mb / 1000).toFixed(1)} GB)
                      </option>
                    )}
                  </For>
                </select>
                <div class="flex gap-2">
                  <button class={btnPrimary} onClick={() => void run(a().ext, "install", a().pick)}>
                    {installsHost(a().ext)
                      ? a().ext.kind === "host"
                        ? "Install uv and the Gradio host"
                        : `Install uv, the Gradio host and ${a().ext.name}`
                      : `Install uv and ${a().ext.name}`}
                  </button>
                  <button class={btn} onClick={() => setAsk(null)}>
                    Not now
                  </button>
                </div>
              </div>
            )}
          </Show>
          <Show when={busy()}>
            <div>
              <button class={btn} onClick={() => abort?.abort()}>
                Cancel
              </button>
            </div>
          </Show>
          <Show when={log.length > 0}>
            <pre class="p-3 rounded-lg bg-gray-900 text-gray-100 text-xs overflow-auto max-h-72 whitespace-pre-wrap">
              {log.join("\n")}
            </pre>
          </Show>
        </div>
      </Loading>
    </Errored>
  );
}
