// GPUView: GPU selector that POSTs to /api/steps/gpu.
// No platform imports, no file writes — all state is Go-side.
import { useState } from "react";

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

export function GPUView({ onComplete }: GPUViewProps) {
  const [choice, setChoice] = useState(GPU_CHOICES[0].value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const r = await fetch("/api/steps/gpu", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ label: choice }),
      });
      if (!r.ok) {
        const text = await r.text();
        setError(`${r.status}: ${text}`);
        return;
      }
      onComplete();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="rounded-xl border border-blue-400 dark:border-blue-600 bg-white dark:bg-gray-900 p-4">
      <div className="font-medium text-sm mb-1">Select GPU / Accelerator</div>
      <p className="text-xs text-gray-500 dark:text-gray-400 mb-4">
        Choose your hardware for PyTorch installation.
      </p>

      <div className="flex flex-col gap-2 mb-4">
        {GPU_CHOICES.map((g) => (
          <label
            key={g.value}
            className="flex items-center gap-2 text-sm cursor-pointer"
          >
            <input
              type="radio"
              name="gpu"
              value={g.value}
              checked={choice === g.value}
              onChange={() => setChoice(g.value)}
              className="accent-blue-600"
            />
            {g.label}
          </label>
        ))}
      </div>

      {error && (
        <div className="mb-3 p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300 text-xs">
          {error}
        </div>
      )}

      <button
        onClick={save}
        disabled={saving}
        className="w-full py-2 px-3 rounded-lg bg-blue-600 hover:bg-blue-500 disabled:bg-gray-400 text-white text-sm font-medium transition-colors cursor-pointer"
      >
        {saving ? "Saving…" : "Continue"}
      </button>
    </div>
  );
}
