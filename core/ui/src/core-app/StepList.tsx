// StepList renders the installation waterfall using only /api/steps/* endpoints.
import { useState } from "react";
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

// Steps that may be skipped (hidden as done, in this browser only; nothing changes on the server).
const SKIPPABLE = new Set(["update"]);
const SKIP_KEY = "tts-skipped-steps";

function loadSkipped(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(SKIP_KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x) => typeof x === "string") : [];
  } catch {
    return [];
  }
}

export function StepList({ statuses, onRefresh }: StepListProps) {
  const [skipped, setSkipped] = useState<string[]>(loadSkipped);
  function setSkip(step: string, on: boolean) {
    const next = on ? [...skipped.filter((x) => x !== step), step] : skipped.filter((x) => x !== step);
    setSkipped(next);
    try {
      localStorage.setItem(SKIP_KEY, JSON.stringify(next));
    } catch {
      // storage unavailable: the skip lasts until the page is reloaded
    }
  }
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

  // A new install (or a finished old one) has no waterfall: the app is launched
  // right away, an update shows only when there is something to say about it,
  // and the system checks live on the Extensions page.
  // An install on the old env gets the same page once that env is installed.
  // The old steps stay only while it is half done, so it can still be finished.
  const legacyPending = statuses.some(
    (s) => (s.step === "mamba" || s.step === "gpu" || s.step === "install") && !isDone(s),
  );
  if (!legacyPending) {
    const upd = statuses.find((s) => s.step === "update");
    const updSkipped = skipped.includes("update") && !upd?.running;
    const showUpdate = !!upd && (updSkipped || !isDone(upd) || !!upd.detail);
    return (
      <div className="flex flex-col gap-4">
        {upd && showUpdate && (
          <StepItem
            step="update"
            title={STEP_META.update.title}
            description={STEP_META.update.description}
            state={isDone(upd) || updSkipped ? "done" : "active"}
            hasStream={STEP_META.update.hasStream}
            running={upd.running}
            detail={upd.detail}
            skipped={updSkipped}
            onSkip={() => setSkip("update", true)}
            onUnskip={() => setSkip("update", false)}
            onComplete={onRefresh}
          />
        )}
        <LaunchStep />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {statuses.map((s, idx) => {
        const meta = STEP_META[s.step] ?? {
          title: s.step,
          description: "",
          hasStream: false,
        };
        const isSkipped = SKIPPABLE.has(s.step) && skipped.includes(s.step) && !s.running;
        const state: "done" | "active" | "pending" = isDone(s) || isSkipped
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
            skipped={isSkipped}
            onSkip={SKIPPABLE.has(s.step) ? () => setSkip(s.step, true) : undefined}
            onUnskip={() => setSkip(s.step, false)}
            onComplete={onRefresh}
          />
        );
      })}
      {allDone && <LaunchStep />}
    </div>
  );
}
