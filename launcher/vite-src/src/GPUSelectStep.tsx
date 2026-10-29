import { useState } from "react";
import { filesystem } from "@neutralinojs/lib";
import { WEBUI_ROOT, joinPath } from "./config";

const GPU_FILE_PATH = joinPath(WEBUI_ROOT, "installer_scripts", ".gpu");

export const GPU_CHOICES = [
  { value: "NVIDIA GPU", label: "NVIDIA GPU" },
  {
    value: "Custom (Manual torch install)",
    label: "Custom (Manual torch install)",
  },
  { value: "Apple M Series Chip", label: "Apple M Series Chip" },
  { value: "AMD GPU (ROCM, Linux only)", label: "AMD GPU (ROCM, Linux only)" },
  { value: "Intel GPU (XPU)", label: "Intel GPU (XPU)" },
  { value: "CPU", label: "CPU" },
  { value: "Cancel", label: "Cancel" },
  {
    value: "Integrated GPU (unsupported)",
    label: "Integrated GPU (unsupported)",
  },
];

interface GPUSelectStepProps {
  onComplete: () => void;
}

export function GPUSelectStep({ onComplete }: GPUSelectStepProps) {
  const [pythonVersion, setPythonVersion] = useState("3.10.11");
  const [gpuChoice, setGpuChoice] = useState(GPU_CHOICES[0].value);
  const [torchVersion, setTorchVersion] = useState("2.11.0");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const saveAndContinue = async () => {
    setSaving(true);
    setError(null);
    try {
      //   const config = JSON.stringify({ pythonVersion, gpuChoice, torchVersion }, null, 2);
      //   await filesystem.writeFile(GPU_CONFIG_PATH, config);
      await filesystem.writeFile(GPU_FILE_PATH, gpuChoice);
      onComplete();
    } catch (e: any) {
      setError(e?.message ?? "Failed to write config");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Python version */}
      <div>
        <label className="block mb-1 text-sm text-gray-700 dark:text-gray-300">
          Python Version
        </label>
        <input
          type="text"
          disabled
          value={pythonVersion}
          onChange={(e) => setPythonVersion(e.target.value)}
          className="w-full p-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 text-sm"
        />
      </div>

      {/* GPU choice */}
      <div>
        <label className="block mb-1 text-sm text-gray-700 dark:text-gray-300">
          GPU / Accelerator
        </label>
        <select
          value={gpuChoice}
          onChange={(e) => setGpuChoice(e.target.value)}
          className="w-full p-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 text-sm"
        >
          {GPU_CHOICES.map((choice) => (
            <option key={choice.value} value={choice.value}>
              {choice.label}
            </option>
          ))}
        </select>
      </div>

      {/* Torch version */}
      <div>
        <label className="block mb-1 text-sm text-gray-700 dark:text-gray-300">
          Torch Version
        </label>
        <input
          type="text"
          disabled
          value={torchVersion}
          onChange={(e) => setTorchVersion(e.target.value)}
          className="w-full p-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 text-sm"
        />
      </div>

      {/* Config preview */}
      {/* <pre className="bg-gray-100 dark:bg-gray-800 rounded-lg p-3 text-xs text-gray-700 dark:text-gray-300 overflow-auto">
        {JSON.stringify({ pythonVersion, gpuChoice, torchVersion }, null, 2)}
      </pre> */}

      {error && (
        <p className="text-xs text-red-600 dark:text-red-400">{error}</p>
      )}

      <button
        onClick={saveAndContinue}
        disabled={saving}
        className="w-full py-2 px-3 rounded-lg font-medium bg-blue-600 hover:bg-blue-500 text-white transition-colors cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed"
      >
        {saving ? "Saving…" : "Save & Continue"}
      </button>
    </div>
  );
}
