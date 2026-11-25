// GPUView: GPU selector that POSTs to /api/steps/gpu.
// No platform imports, no file writes — all state is Go-side.
import { action, createOptimistic, createSignal, For, Show } from "solid-js";
import { client } from "./client";

const GPU_CHOICES = [
  { value: "NVIDIA GPU", label: "NVIDIA GPU" },
  { value: "AMD GPU (ROCM, Linux only)", label: "AMD GPU (ROCM, Linux only)" },
  { value: "Apple M Series Chip", label: "Apple M Series Chip" },
  { value: "Intel GPU (XPU)", label: "Intel GPU (XPU)" },
  { value: "CPU", label: "CPU" },
  { value: "Custom (Manual torch install)", label: "Custom (Manual torch install)" },
];

interface GPUViewProps {
  onComplete: () => void;
}

export function GPUView(props: GPUViewProps) {
  const [choice, setChoice] = createSignal(GPU_CHOICES[0].value);
  // "Saving…" is an optimistic flag: it is true while the action runs and
  // reverts on its own when the transition ends, so there is no finally.
  const [saving, setSaving] = createOptimistic(false);
  const [error, setError] = createSignal<string | null>(null);

  // Writes to plain signals inside an action land when the action settles, so
  // the error (or its clearing) appears together with "Saving…" ending.
  const save = action(function* () {
    setSaving(true);
    setError(null);
    try {
      const r: Response = yield client("/api/steps/gpu", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ label: choice() }),
      });
      if (!r.ok) {
        const text: string = yield r.text();
        setError(`${r.status}: ${text}`);
        return;
      }
      props.onComplete();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    }
  });

  return (
    <div class="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <div class="font-medium text-sm mb-1">Select GPU / Accelerator</div>
      <p class="text-xs text-gray-500 dark:text-gray-400 mb-4">
        Choose your hardware for PyTorch installation.
      </p>

      <div class="flex flex-col gap-2 mb-4">
        <For each={GPU_CHOICES}>
          {(g) => (
            <label class="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="radio"
                name="gpu"
                value={g.value}
                checked={choice() === g.value}
                onChange={() => setChoice(g.value)}
                class="accent-blue-600"
              />
              {g.label}
            </label>
          )}
        </For>
      </div>

      <Show when={error()}>
        {(message) => (
          <div class="mb-3 p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-xs">
            {message()}
          </div>
        )}
      </Show>

      <button
        onClick={() => save()}
        disabled={saving()}
        class="w-full py-2 px-3 rounded-lg bg-blue-600 hover:bg-blue-500 disabled:bg-gray-400 text-white text-sm font-medium transition-colors cursor-pointer"
      >
        {saving() ? "Saving…" : "Continue"}
      </button>
    </div>
  );
}
