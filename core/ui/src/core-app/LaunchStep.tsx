// LaunchStep: after all installation steps are done, drives the app lifecycle
// via /api/app/* — no /api/steps/run involvement.
import {
  createEffect,
  createMemo,
  createSignal,
  createStore,
  For,
  Loading,
  onSettled,
  Show,
} from "solid-js";
import { client } from "./client";

interface ProcessStatus {
  name: string;
  running: boolean;
  state?: string; // starting, ready, exited, failed, ...
  error?: string; // why it failed (for example: no Python environment)
  warning?: string; // carried on despite, e.g. a missing GPU label
}

interface LanAnswer {
  lan: boolean;
  warning?: string;
  urls?: string[];
}

export function LaunchStep() {
  const [processes, setProcesses] = createSignal<ProcessStatus[]>([]);
  const [appURL, setAppURL] = createSignal("");
  const [launched, setLaunched] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [diagLines, setDiagLines] = createStore<string[]>([]);
  const [diagRunning, setDiagRunning] = createSignal(false);
  let es: EventSource | undefined;
  let diagEs: EventSource | undefined;

  // LAN mode (TTS_GO_PROXY_LISTEN=lan): show the same warning the launcher logs (R2-8).
  // A failed read is "not LAN", as before.
  const lan = createMemo<LanAnswer>(() =>
    client("/api/app/lan")
      .then((r) => (r.ok ? (r.json() as Promise<LanAnswer>) : { lan: false }))
      .catch(() => ({ lan: false })),
  );

  // Poll the app link while any process runs. The effect re-runs only when
  // "anything running" flips, and its cleanup is what stops the timer.
  const anyRunning = createMemo(() => processes().some((p) => p.running));
  createEffect(anyRunning, (running) => {
    if (!running) return;
    const timer = setInterval(async () => {
      try {
        const r = await client("/api/app/link");
        if (!r.ok) return;
        const data = (await r.json()) as { url: string };
        if (data.url) setAppURL(data.url);
      } catch {
        // ignore network errors during polling
      }
    }, 2000);
    return () => clearInterval(timer);
  });

  // Cleanup on unmount.
  onSettled(() => () => {
    es?.close();
    diagEs?.close();
  });

  function subscribeStatus() {
    if (es) return;
    const source = new EventSource("/api/app/status/stream");
    es = source;
    source.onmessage = (e) => {
      try {
        setProcesses(JSON.parse(e.data) as ProcessStatus[]);
      } catch {
        // ignore parse errors
      }
    };
    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        es = undefined;
      }
    };
  }

  async function launch() {
    setError(null);
    try {
      const r = await client("/api/app/start", { method: "POST" });
      // 409 means already running — subscribe to its status stream.
      if (!r.ok && r.status !== 409) {
        setError(`Failed to start: ${r.status}: ${await r.text()}`);
        return;
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      return;
    }
    setLaunched(true);
    subscribeStatus();
  }

  async function stop() {
    await client("/api/app/stop", { method: "POST" });
  }

  async function restart() {
    setAppURL("");
    await client("/api/app/restart", { method: "POST" });
  }

  async function runDiagnostic() {
    if (diagRunning()) return;
    setDiagLines(() => []);
    setDiagRunning(true);
    diagEs?.close();
    await client("/api/app/diagnostic", { method: "POST" });
    const source = new EventSource("/api/app/diagnostic/stream");
    diagEs = source;
    source.onmessage = (e) =>
      setDiagLines((lines) => {
        lines.push(e.data);
      });
    source.addEventListener("done", () => {
      setDiagRunning(false);
      source.close();
      diagEs = undefined;
    });
    source.onerror = () => {
      setDiagRunning(false);
      source.close();
      diagEs = undefined;
    };
  }

  const allStopped = () => launched() && processes().length > 0 && !anyRunning();

  return (
    <div class="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <h2 class="text-base font-semibold mb-2">Launch WebUI</h2>

      <Loading fallback={null}>
        <Show when={lan().lan}>
          <div
            role="alert"
            class="mb-3 p-3 rounded-lg border border-amber-400 bg-amber-50 dark:bg-amber-950/40 dark:border-amber-700 text-sm text-amber-900 dark:text-amber-200"
          >
            <p class="font-medium">Reachable from your network, with no password</p>
            <p class="mt-1">{lan().warning}</p>
            <Show when={lan().urls?.length}>
              <ul class="mt-1 font-mono text-xs">
                <For each={lan().urls}>{(u) => <li>{u}</li>}</For>
              </ul>
            </Show>
          </div>
        </Show>
      </Loading>

      <Show when={error()}>
        {(message) => (
          <p class="text-sm text-red-600 dark:text-red-400 mb-2">{message()}</p>
        )}
      </Show>

      <Show when={!launched()}>
        <button
          onClick={() => void launch()}
          class="px-4 py-2 rounded bg-blue-600 text-white hover:bg-blue-700 text-sm"
        >
          Launch WebUI
        </button>
      </Show>

      <Show when={launched() && processes().length > 0}>
        <ul class="mt-2 space-y-1">
          <For each={processes()} keyed={(p) => p.name}>
            {(p) => (
              <li class="flex items-center gap-2 text-sm">
                <span
                  class={[
                    "w-2 h-2 rounded-full",
                    p().running ? "bg-green-500" : "bg-gray-400",
                  ]}
                />
                <span class="font-mono">{p().name}</span>
                <span class="text-gray-500 dark:text-gray-400">
                  {p().running ? "running" : (p().state ?? "stopped")}
                  {p().error ? `: ${p().error}` : ""}
                  {p().warning ? ` (warning: ${p().warning})` : ""}
                </span>
              </li>
            )}
          </For>
        </ul>
      </Show>

      <Show when={appURL()}>
        {(url) => (
          <a
            href={url()}
            target="_blank"
            rel="noreferrer"
            class="mt-3 inline-flex px-4 py-2 rounded bg-green-600 text-white hover:bg-green-700 text-sm"
          >
            Open WebUI →
          </a>
        )}
      </Show>

      <Show when={allStopped()}>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">App stopped.</p>
      </Show>

      <Show when={launched()}>
        <div class="mt-3 flex gap-2">
          <button
            onClick={() => void stop()}
            class="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            Stop
          </button>
          <button
            onClick={() => void restart()}
            class="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            Restart
          </button>
          <button
            onClick={() => void runDiagnostic()}
            disabled={diagRunning()}
            class="px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-sm hover:bg-gray-100 dark:hover:bg-gray-800 disabled:opacity-50"
          >
            {diagRunning() ? "Running Diagnostic…" : "Diagnostic"}
          </button>
        </div>
      </Show>

      <Show when={diagLines.length > 0}>
        <pre class="mt-3 p-2 rounded bg-gray-100 dark:bg-gray-800 text-xs font-mono overflow-auto max-h-48">
          {diagLines.join("\n")}
        </pre>
      </Show>
    </div>
  );
}
