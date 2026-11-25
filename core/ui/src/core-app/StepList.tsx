// StepList renders the installation waterfall using only /api/steps/* endpoints.
import { createMemo, For, Match, Show, Switch } from "solid-js";
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

// A running step is never shown done, and while any step runs no other step
// can be started: the server answers 409 anyway (R2-3).
const isDone = (s: StepStatus) => s.done && !s.running;

export function StepList(props: StepListProps) {
  const anyRunning = createMemo(() => props.statuses.some((s) => s.running));
  // The active step is the first one that is not done.
  const activeIdx = createMemo(() => props.statuses.findIndex((s) => !isDone(s)));
  // Update is optional: an update waiting, or one that could not be checked
  // (offline), never hides the Start button.
  const allDone = createMemo(
    () =>
      props.statuses.length > 0 &&
      props.statuses.filter((s) => s.step !== "update").every(isDone),
  );

  return (
    <div class="flex flex-col gap-4">
      {/* Keyed by step name, not by object: every refresh brings new objects,
          but a row keeps its logs and its open stream across a refresh. */}
      <For each={props.statuses} keyed={(s) => s.step}>
        {(s, idx) => {
          const meta = () =>
            STEP_META[s().step] ?? { title: s().step, description: "", hasStream: false };
          const state = (): "done" | "active" | "pending" =>
            isDone(s())
              ? "done"
              : idx() === activeIdx() && (!anyRunning() || s().running)
                ? "active"
                : "pending";

          // The active gpu/preflight step swaps to its own view; when it is no
          // longer active the row falls back to a plain StepItem (a new one).
          return (
            <Switch
              fallback={
                <StepItem
                  step={s().step}
                  title={meta().title}
                  description={meta().description}
                  state={state()}
                  hasStream={meta().hasStream}
                  running={s().running}
                  detail={s().detail}
                  onComplete={props.onRefresh}
                />
              }
            >
              <Match when={state() === "active" && s().step === "gpu"}>
                <GPUView onComplete={props.onRefresh} />
              </Match>
              <Match when={state() === "active" && s().step === "preflight"}>
                <PreflightView onComplete={props.onRefresh} />
              </Match>
            </Switch>
          );
        }}
      </For>
      <Show when={allDone()}>
        <LaunchStep />
      </Show>
    </div>
  );
}
