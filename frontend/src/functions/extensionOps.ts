import React from "react";

/**
 * Install and uninstall of catalog extensions through the Go routes under
 * /api/go-data/extensions (core_extensions_install.go). The control API answers
 * NDJSON: {line}, {plan}, {step}, then {done, ok}. Only a package_name is sent;
 * Go looks it up in the catalog.
 */
export type Op = "install" | "uninstall";

export type Run = {
  op: Op;
  name: string;
  proxy: string;
  lines: string[];
  step: string;
  done: boolean;
  ok: boolean;
  needsGpu: boolean;
  /** "2 of 3" while several entries are installed one after another. */
  progress: string;
};

export type Job = { op: Op; name: string; package_name: string; proxy?: string };

const blank = (job: Job, progress: string): Run => ({
  op: job.op,
  name: job.name,
  proxy: job.proxy ?? "",
  lines: [],
  step: "",
  done: false,
  ok: false,
  needsGpu: false,
  progress,
});

type Patch = (r: Run) => Run;

/** Streams one operation into `onUpdate`; resolves with whether it succeeded. */
async function streamOp(job: Job, onUpdate: (p: Patch) => void): Promise<boolean> {
  const response = await fetch(`/api/go-data/extensions/${job.op}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ package_name: job.package_name }),
  });
  if (!response.ok || !response.body) {
    const body = await response
      .json()
      .catch(() => ({} as { error?: string; needs?: string }));
    onUpdate((r) => ({
      ...r,
      lines: [...r.lines, `Error: ${body.error ?? `HTTP ${response.status}`}`],
      needsGpu: response.status === 409 && body.needs === "gpu_choice",
    }));
    return false;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let ok = false;
  const handle = (text: string) => {
    if (!text.trim()) return;
    const msg = JSON.parse(text);
    if (typeof msg.line === "string") {
      onUpdate((r) => ({ ...r, lines: [...r.lines, msg.line] }));
    }
    if (msg.step?.id) {
      onUpdate((r) => ({ ...r, step: `${msg.step.id}: ${msg.step.state}` }));
    }
    if (msg.done) ok = !!msg.ok;
  };
  for (;;) {
    const { value, done } = await reader.read();
    if (value) buffer += decoder.decode(value, { stream: !done });
    const parts = buffer.split("\n");
    buffer = parts.pop() ?? "";
    parts.forEach(handle);
    if (done) {
      handle(buffer);
      break;
    }
  }
  return ok;
}

/**
 * The run shown in the dialog: `start` runs the jobs one after another (a stop
 * on the first failure), `view` is the last run (it stays while the dialog
 * fades), `busy` is true while one is running.
 */
export const useExtensionRuns = (onFinished?: () => void) => {
  const [run, setRun] = React.useState<Run | null>(null);
  const last = React.useRef<Run | null>(null);
  if (run) last.current = run;

  const start = React.useCallback(
    async (jobs: Job[]) => {
      let allOk = true;
      for (let i = 0; i < jobs.length && allOk; i++) {
        const job = jobs[i];
        setRun(blank(job, jobs.length > 1 ? `${i + 1} of ${jobs.length}` : ""));
        const update = (p: Patch) => setRun((r) => (r ? p(r) : r));
        try {
          allOk = await streamOp(job, update);
        } catch (e) {
          allOk = false;
          update((r) => ({
            ...r,
            lines: [...r.lines, `Error: ${e instanceof Error ? e.message : String(e)}`],
          }));
        }
        const finishedOk = allOk;
        setRun((r) =>
          r
            ? {
                ...r,
                // Between two jobs of a series the dialog stays open and working.
                done: !finishedOk || i === jobs.length - 1,
                ok: finishedOk,
                lines:
                  !finishedOk && !r.lines.some((l) => l.startsWith("Error:"))
                    ? [...r.lines, "It did not finish."]
                    : r.lines,
              }
            : r
        );
        onFinished?.();
      }
    },
    [onFinished]
  );

  return {
    run,
    view: run ?? last.current,
    busy: !!run && !run.done,
    start,
    close: () => setRun(null),
  };
};
