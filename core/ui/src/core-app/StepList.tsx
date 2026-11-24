// StepList renders the installation waterfall using only /api/steps/* endpoints.
import type { StepStatus } from "./CoreApp";
import { StepItem } from "./StepItem";
import { GPUView } from "./GPUView";
import { LaunchStep } from "./LaunchStep";
import { PreflightView } from "./PreflightView";

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
    description: "Download the latest release and replace the program, the UI and the sources (your data is not touched)",
    hasStream: true,
  },
};

export function StepList({ statuses, onRefresh }: StepListProps) {
  // A running step is never shown done, and while any step runs no other step
  // can be started: the server answers 409 anyway (R2-3).
  const isDone = (s: StepStatus) => s.done && !s.running;
  const anyRunning = statuses.some((s) => s.running);
  // The active step is the first one that is not done.
  const activeIdx = statuses.findIndex((s) => !isDone(s));
  // Update is optional: an update waiting, or one that could not be checked
  // (offline), never hides the Start button.
  const allDone =
    statuses.length > 0 &&
    statuses.filter((s) => s.step !== "update").every(isDone);

  return (
    <div className="flex flex-col gap-4">
      {statuses.map((s, idx) => {
        const meta = STEP_META[s.step] ?? {
          title: s.step,
          description: "",
          hasStream: false,
        };
        const state: "done" | "active" | "pending" = isDone(s)
          ? "done"
          : idx === activeIdx && (!anyRunning || s.running)
            ? "active"
            : "pending";

        if (state === "active" && s.step === "gpu") {
          return (
            <GPUView key={s.step} onComplete={onRefresh} />
          );
        }

        if (state === "active" && s.step === "preflight") {
          return (
            <PreflightView key={s.step} onComplete={onRefresh} />
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
            running={s.running}
            detail={s.detail}
            onComplete={onRefresh}
          />
        );
      })}
      {allDone && <LaunchStep />}
    </div>
  );
}
