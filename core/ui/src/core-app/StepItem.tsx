// StepItem: renders one installation step, calling only /api/steps/*.
import { createEffect, createSignal, createStore, For, onSettled, Show } from "solid-js";
import { client } from "./client";

interface StepItemProps {
  step: string;
  title: string;
  description: string;
  state: "done" | "active" | "pending";
  hasStream: boolean;
  running?: boolean; // server reports a proc is already running (reattach on mount)
  detail?: string; // a line from the server: an update waiting, or why it could not be checked
  onComplete: () => void;
}

export function StepItem(props: StepItemProps) {
  const [isRunning, setIsRunning] = createSignal(false);
  const [logs, setLogs] = createStore<string[]>([]);
  const [error, setError] = createSignal<string | null>(null);
  // The stream is a push source, not a read: it stays an imperative handle.
  let es: EventSource | undefined;
  let logEnd: HTMLDivElement | undefined;

  // Auto-scroll to the bottom as new log lines arrive.
  createEffect(
    () => logs.length,
    () => {
      logEnd?.scrollIntoView({ behavior: "smooth" });
    },
  );

  // Tear down EventSource when step becomes inactive.
  createEffect(
    () => props.state,
    (state) => {
      if (state !== "active") {
        es?.close();
        es = undefined;
      }
    },
  );

  // Reattach to an in-progress stream on page reload.
  onSettled(() => {
    if (props.state === "active" && props.running) {
      setIsRunning(true);
      openStream();
    }
  });

  function openStream() {
    if (es) return;
    const source = new EventSource(`/api/steps/${props.step}/stream`);
    es = source;
    source.onmessage = (e) =>
      setLogs((lines) => {
        lines.push(e.data);
      });
    source.addEventListener("done", (e) => {
      source.close();
      es = undefined;
      const payload = JSON.parse((e as MessageEvent).data ?? "{}") as {
        exitCode?: number;
      };
      if ((payload.exitCode ?? 0) === 0) {
        props.onComplete();
      } else {
        setError(`Step exited with code ${payload.exitCode}`);
        setIsRunning(false);
      }
    });
    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        setIsRunning(false);
      }
    };
  }

  async function runStep() {
    setIsRunning(true);
    setError(null);
    setLogs(() => []);

    let responseBody: { started?: boolean } = {};
    try {
      const r = await client(`/api/steps/${props.step}`, { method: "POST" });
      if (!r.ok) {
        const text = await r.text();
        setError(`POST /api/steps/${props.step} → ${r.status}: ${text}`);
        setIsRunning(false);
        return;
      }
      responseBody = (await r.json()) as { started?: boolean };
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      setIsRunning(false);
      return;
    }

    // If the server says started:false (e.g. update already up-to-date), complete immediately.
    if (props.hasStream && responseBody.started === false) {
      setIsRunning(false);
      props.onComplete();
      return;
    }

    if (props.hasStream) {
      openStream();
    } else {
      setIsRunning(false);
      props.onComplete();
    }
  }

  const cardClass = () =>
    props.state === "done" && props.detail
      ? "border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-950/30"
      : props.state === "done"
        ? "border-green-300 dark:border-green-800 bg-green-50 dark:bg-green-950/30"
        : props.state === "active"
          ? "border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900"
          : "border-gray-200 dark:border-gray-800 opacity-50";

  return (
    <div class={["rounded-xl border p-4 transition-colors", cardClass()]}>
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <Show when={props.state === "done" && !props.detail}>
              <span class="text-green-600 dark:text-green-400 text-sm">✓</span>
            </Show>
            <span class="font-medium text-sm">{props.title}</span>
          </div>
          <Show when={props.description}>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
              {props.description}
            </p>
          </Show>
          <Show when={props.detail}>
            <p class="text-xs text-amber-700 dark:text-amber-400 mt-1">{props.detail}</p>
          </Show>
        </div>
        <Show when={props.state === "active"}>
          <button
            onClick={() => void runStep()}
            disabled={isRunning()}
            class={[
              "shrink-0 py-1.5 px-3 rounded-lg text-sm font-medium transition-colors cursor-pointer",
              isRunning()
                ? "bg-gray-200 dark:bg-gray-700 text-gray-400 cursor-wait"
                : "bg-blue-600 hover:bg-blue-500 text-white",
            ]}
          >
            {isRunning() ? "Running..." : "Run"}
          </button>
        </Show>
      </div>

      <Show when={error()}>
        {(message) => (
          <div class="mt-3 p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-xs font-mono">
            {message()}
          </div>
        )}
      </Show>

      <Show when={logs.length > 0}>
        <div class="mt-3 max-h-48 overflow-y-auto bg-gray-950 rounded-lg p-3">
          {/* Append-only lines: non-keyed, so a repeated line is its own row. */}
          <For each={logs} keyed={false}>
            {(line) => (
              <div class="text-xs font-mono text-green-400 whitespace-pre-wrap">
                {line()}
              </div>
            )}
          </For>
          <div ref={(el) => (logEnd = el)} />
        </div>
      </Show>
    </div>
  );
}
