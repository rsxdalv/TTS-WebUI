// CoreApp: launcher UI that calls only /api/steps/* and /api/app/*.
// No Neutralino, no NL_* globals, no platform/ imports.
import { createMemo, createSignal, For, Loading, onSettled, refresh, Show } from "solid-js";
import { StepList } from "./StepList";
import { DarkThemeProvider, ThemeToggle } from "../DarkThemeProvider";
import { client } from "./client";
import { ExtensionsView } from "./ExtensionsView";
import { Notice } from "./Notice";

type Page = "setup" | "extensions";

const pageFromHash = (): Page =>
  window.location.hash === "#extensions" ? "extensions" : "setup";

export interface StepStatus {
  step: string;
  done: boolean;
  running?: boolean;
  detail?: string;
}

// A failed fetch is an answer, not a throw: it must not discard the list the
// page already shows (the banner sits above it), so it is carried as a value.
type StatusAnswer = { statuses: StepStatus[] } | { error: string };

async function fetchStatus(): Promise<StatusAnswer> {
  try {
    const r = await client("/api/steps/status");
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return { statuses: (await r.json()) as StepStatus[] };
  } catch (e: unknown) {
    return { error: e instanceof Error ? e.message : String(e) };
  }
}

export function CoreApp() {
  const [page, setPage] = createSignal<Page>(pageFromHash());

  onSettled(() => {
    const onHash = () => setPage(pageFromHash());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  });

  // One GET on mount; refresh() asks the same question again (a step finished).
  const answer = createMemo(fetchStatus);
  const setup = createMemo<{ statuses: StepStatus[] | null; error: string | null }>(
    (prev) => {
      const a = answer();
      return "statuses" in a
        ? { statuses: a.statuses, error: null }
        : { statuses: prev?.statuses ?? null, error: a.error };
    },
  );
  const onRefresh = () => refresh(answer);

  return (
    <DarkThemeProvider>
      <div class="min-h-screen bg-gray-50 dark:bg-gray-950 text-gray-900 dark:text-gray-100">
        <header class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-800">
          <div class="flex items-center gap-6">
            <h1 class="text-lg font-semibold">TTS WebUI</h1>
            <nav class="flex gap-4 text-sm">
              <For each={["setup", "extensions"] as const}>
                {(p) => (
                  <a
                    href={p === "setup" ? "#" : `#${p}`}
                    class={
                      page() === p
                        ? "font-semibold underline underline-offset-4"
                        : "text-gray-500 dark:text-gray-400 hover:underline"
                    }
                  >
                    {p === "setup" ? "Setup" : "Extensions"}
                  </a>
                )}
              </For>
            </nav>
          </div>
          <ThemeToggle />
        </header>
        <main class={["mx-auto px-4 py-8", page() === "extensions" ? "max-w-4xl" : "max-w-2xl"]}>
          <Notice />
          <Show when={page() === "extensions"}>
            <ExtensionsView />
          </Show>
          <Show when={page() === "setup"}>
            <Loading
              fallback={
                <p class="text-gray-500 dark:text-gray-400 text-sm">
                  Detecting installation state…
                </p>
              }
            >
              <Show when={setup().error}>
                {(error) => (
                  <div class="mb-6 p-4 rounded-lg bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-sm">
                    Cannot reach Go core: {error()}. Is the binary running?
                  </div>
                )}
              </Show>
              <Show when={setup().statuses}>
                {(statuses) => <StepList statuses={statuses()} onRefresh={onRefresh} />}
              </Show>
            </Loading>
          </Show>
        </main>
      </div>
    </DarkThemeProvider>
  );
}
