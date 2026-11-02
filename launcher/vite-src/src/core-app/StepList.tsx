// StepList renders the installation waterfall using only /api/steps/* endpoints.
import type { StepStatus } from "./CoreApp";
import { StepItem } from "./StepItem";
import { GPUView } from "./GPUView";

interface StepListProps {
  statuses: StepStatus[];
  onRefresh: () => void;
}

const STEP_META: Record<string, { title: string; description: string; hasStream: boolean }> = {
  download: {
    title: "Download & Extract Release",
    description: "Fetch the TTS WebUI installer package",
    hasStream: true,
  },
  preflight: {
    title: "System Pre-flight Checks",
    description: "Verify system requirements",
    hasStream: false,
  },
  mamba: {
    title: "Init Mamba Environment",
    description: "Download and set up the micromamba package manager",
    hasStream: true,
  },
  gpu: {
    title: "Select GPU / Accelerator",
    description: "Choose your hardware for PyTorch installation",
    hasStream: false,
  },
  install: {
    title: "Install WebUI",
    description: "Create the Python environment and install all dependencies",
    hasStream: true,
  },
  update: {
    title: "Update WebUI",
    description: "Pull the latest changes and re-install dependencies if needed",
    hasStream: true,
  },
  run: {
    title: "Launch WebUI",
    description: "Start the TTS WebUI server",
    hasStream: true,
  },
};

export function StepList({ statuses, onRefresh }: StepListProps) {
  // The active step is the first one that is not done.
  const activeIdx = statuses.findIndex((s) => !s.done);

  return (
    <div className="flex flex-col gap-4">
      {statuses.map((s, idx) => {
        const meta = STEP_META[s.step] ?? {
          title: s.step,
          description: "",
          hasStream: false,
        };
        const state: "done" | "active" | "pending" =
          s.done ? "done" : idx === activeIdx ? "active" : "pending";

        if (state === "active" && s.step === "gpu") {
          return (
            <GPUView key={s.step} onComplete={onRefresh} />
          );
        }

        return (
          <StepItem
            key={s.step}
            step={s.step}
            title={meta.title}
            description={meta.description}
            state={state}
            hasStream={meta.hasStream}
            onComplete={onRefresh}
          />
        );
      })}
    </div>
  );
}
