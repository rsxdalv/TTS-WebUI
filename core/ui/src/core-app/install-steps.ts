// The steps of an install, as the launcher streams them.
//
// The NDJSON answer of /api/ext/op/install carries, besides {"line": "..."}:
//   {"plan": [{"id": "uv", "label": "Download uv"}, ...]}   the steps, in order, all pending
//   {"step": {"id": "torch", "state": "running", "done": N, "total": M,
//             "estimated": true, "detail": "..."}}            one step changed
// Everything but id is optional in a "step" message; it is merged into the step.

export type StepState = "pending" | "running" | "done" | "skipped" | "failed";

export interface StepInfo {
  id: string;
  label: string;
  state: StepState;
  /** Bytes done and in all, when known. */
  done?: number;
  total?: number;
  /** done and total count this (packages) when set, bytes when not. */
  unit?: string;
  /** total is a guess (uv prints no progress; the cache folder's growth is measured). */
  estimated?: boolean;
  detail?: string;
  startedAt?: number;
  finishedAt?: number;
}

export type StepMessage =
  | { plan: { id: string; label: string }[] }
  | { step: Partial<StepInfo> & { id: string } };

export function applyStepMessage(steps: StepInfo[], msg: StepMessage, now = Date.now()): StepInfo[] {
  if ("plan" in msg) {
    // A plan may be sent again; a step that is already known keeps its state.
    return msg.plan.map(
      (p) => steps.find((s) => s.id === p.id) ?? { id: p.id, label: p.label, state: "pending" as StepState },
    );
  }
  const upd = msg.step;
  const i = steps.findIndex((s) => s.id === upd.id);
  const prev: StepInfo = i >= 0 ? steps[i] : { id: upd.id, label: upd.label ?? upd.id, state: "pending" };
  const next: StepInfo = { ...prev, ...upd };
  // The launcher leaves out what is empty. A new size is a new kind of progress.
  if (upd.total !== undefined) {
    next.unit = upd.unit;
    next.estimated = upd.estimated;
  }
  // A step another part of the install already did is "already there" again
  // for the part that checks; it stays done.
  if (prev.state === "done" && upd.state === "skipped") next.state = "done";
  if (next.state === "running" && !next.startedAt) next.startedAt = now;
  if ((next.state === "done" || next.state === "failed" || next.state === "skipped") && !next.finishedAt) {
    next.finishedAt = now;
  }
  if (i >= 0) {
    const out = steps.slice();
    out[i] = next;
    return out;
  }
  return [...steps, next];
}

export function humanBytes(n: number): string {
  if (n >= 1 << 30) return `${(n / (1 << 30)).toFixed(1)} GB`;
  if (n >= 1 << 20) return `${Math.round(n / (1 << 20))} MB`;
  if (n >= 1 << 10) return `${Math.round(n / (1 << 10))} KB`;
  return `${n} B`;
}

export function humanDuration(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s`;
  return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
}
