// PreflightView: runs POST /api/steps/preflight, shows check results and fix buttons.
import {
  affects,
  createEffect,
  createMemo,
  createSignal,
  For,
  isPending,
  Loading,
  Match,
  refresh,
  Show,
  Switch,
} from "solid-js";
import { client } from "./client";

interface PreflightCheck {
  id: string;
  status: "pass" | "warn" | "fail";
  message?: string;
  fix_action?: string;
}

interface PreflightResponse {
  checks: PreflightCheck[];
  ok: boolean;
}

interface FixResponse {
  ok: boolean;
  message?: string;
}

interface PreflightViewProps {
  onComplete: () => void;
}

const failed = (message: string): PreflightResponse => ({
  checks: [{ id: "error", status: "fail", message }],
  ok: false,
});

async function runChecks(): Promise<PreflightResponse> {
  try {
    const r = await client("/api/steps/preflight", { method: "POST" });
    if (!r.ok) {
      const text = await r.text();
      return failed(`Server error ${r.status}: ${text}`);
    }
    return (await r.json()) as PreflightResponse;
  } catch (e: unknown) {
    return failed(e instanceof Error ? e.message : String(e));
  }
}

const readableId = (id: string) =>
  id.replace(/([A-Z])/g, " $1").replace(/^./, (s) => s.toUpperCase()).trim();

function StatusIcon(props: { status: PreflightCheck["status"] }) {
  return (
    <Switch fallback={<span class="text-red-600 dark:text-red-400">✗</span>}>
      <Match when={props.status === "pass"}>
        <span class="text-green-600 dark:text-green-400">✓</span>
      </Match>
      <Match when={props.status === "warn"}>
        <span class="text-yellow-500">⚠</span>
      </Match>
    </Switch>
  );
}

function RetryButton(props: { busy: boolean; onClick: () => void }) {
  return (
    <button
      onClick={() => props.onClick()}
      disabled={props.busy}
      class="py-1.5 px-3 rounded-lg text-sm font-medium bg-blue-600 hover:bg-blue-500 disabled:bg-gray-400 text-white transition-colors cursor-pointer"
    >
      {props.busy ? "Checking…" : "Retry"}
    </button>
  );
}

export function PreflightView(props: PreflightViewProps) {
  // The POST is the read: the first run happens on mount, and Retry re-asks it.
  const result = createMemo(runChecks);
  const [fixMessages, setFixMessages] = createSignal<Record<string, string>>({});

  // Every run that comes back ok moves the waterfall on.
  createEffect(
    () => result().ok,
    (ok) => {
      if (ok) props.onComplete();
    },
  );

  // Retry is a change the user asked for, so declare it: the button reads
  // "Checking…" while the old results stay on screen.
  const retry = () => {
    setFixMessages({});
    affects(result);
    refresh(result);
  };

  async function runFix(fixAction: string, id: string) {
    try {
      const r = await client(`/api/steps/preflight/fix/${fixAction}`, { method: "POST" });
      const data = (await r.json()) as FixResponse;
      setFixMessages((prev) => ({ ...prev, [id]: data.message ?? (data.ok ? "Fixed!" : "Failed.") }));
    } catch (e: unknown) {
      setFixMessages((prev) => ({ ...prev, [id]: e instanceof Error ? e.message : String(e) }));
    }
  }

  return (
    <div class="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <div class="font-medium text-sm mb-1">System Pre-flight Checks</div>
      <p class="text-xs text-gray-500 dark:text-gray-400 mb-4">
        Verify system requirements before installation.
      </p>

      <Loading
        fallback={
          <>
            <p class="text-xs text-gray-500 dark:text-gray-400">Running checks…</p>
            <RetryButton busy={true} onClick={retry} />
          </>
        }
      >
        <Show when={result().checks.length > 0}>
          <div class="flex flex-col gap-3 mb-4">
            <For each={result().checks} keyed={(c) => c.id}>
              {(c) => (
                <div>
                  <div class="flex items-center gap-2 text-sm">
                    <StatusIcon status={c().status} />
                    <span class="font-medium">{readableId(c().id)}</span>
                  </div>
                  <Show when={c().message}>
                    {(message) => (
                      <p class="ml-5 text-xs text-gray-500 dark:text-gray-400 mt-0.5">{message()}</p>
                    )}
                  </Show>
                  <Show when={fixMessages()[c().id]}>
                    {(message) => (
                      <p class="ml-5 text-xs text-yellow-600 dark:text-yellow-400 mt-0.5">
                        {message()}
                      </p>
                    )}
                  </Show>
                  <Show when={c().fix_action}>
                    {(fix) => (
                      <button
                        onClick={() => void runFix(fix(), c().id)}
                        class="ml-5 mt-1 text-xs px-2 py-1 rounded bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 hover:bg-yellow-200 dark:hover:bg-yellow-800 cursor-pointer"
                      >
                        Fix
                      </button>
                    )}
                  </Show>
                </div>
              )}
            </For>
          </div>
        </Show>

        <Show when={!result().ok && result().checks.length > 0}>
          <p class="text-xs text-red-600 dark:text-red-400 mb-3">
            Fix the issues above to continue.
          </p>
        </Show>

        <RetryButton busy={isPending(() => result())} onClick={retry} />
      </Loading>
    </div>
  );
}
