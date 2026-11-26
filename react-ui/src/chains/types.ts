/**
 * The chain graph: data only. A saved graph never carries code or secrets.
 * See documentation/NODE-GRAPH-SPEC.md.
 */

export type ValueType = "audio" | "text" | "number";

/** An audio value: the shape the Gradio functions already return, trimmed. */
export type AudioValue = { url: string; name?: string };

export type Value = AudioValue | string | number | null | undefined;

export type PortSpec = { type: ValueType; label?: string; optional?: boolean };

export type ParamSpec =
  | { key: string; label: string; kind: "text"; default: string }
  | { key: string; label: string; kind: "textarea"; default: string }
  | {
      key: string;
      label: string;
      kind: "number";
      default: number;
      min?: number;
      max?: number;
      step?: number;
    }
  | {
      key: string;
      label: string;
      kind: "slider";
      default: number;
      min: number;
      max: number;
      step: number;
    }
  | {
      key: string;
      label: string;
      kind: "select";
      default: string;
      options: string[];
    }
  | { key: string; label: string; kind: "toggle"; default: boolean };

export type Params = Record<string, string | number | boolean>;

export type RunCtx = {
  signal: AbortSignal;
  progress: (fraction: number, text?: string) => void;
};

export type NodeDef = {
  /** Unique, dotted: "tts.kokoro", "fx.gain". */
  type: string;
  label: string;
  category: "Source" | "Generate" | "Convert" | "Process" | "Output";
  inputs: Record<string, PortSpec>;
  outputs: Record<string, PortSpec>;
  params: ParamSpec[];
  /**
   * Nodes with the same lane run one at a time (a model on one GPU, one
   * Gradio host). Nodes without a lane run as soon as their inputs are ready.
   */
  lane?: string;
  /** False when a run can differ with the same inputs (a random seed). */
  cacheable?: (params: Params) => boolean;
  run: (
    ctx: RunCtx,
    inputs: Record<string, Value>,
    params: Params
  ) => Promise<Record<string, Value>>;
};

export type NodeInstance = {
  id: string;
  type: string;
  x: number;
  y: number;
  params: Params;
  bypass?: boolean;
};

export type Edge = {
  id: string;
  from: [nodeId: string, port: string];
  to: [nodeId: string, port: string];
};

export type Graph = {
  version: 1;
  nodes: NodeInstance[];
  edges: Edge[];
};

export type NodeStatus =
  | "idle"
  | "queued"
  | "running"
  | "done"
  | "cached"
  | "error"
  | "skipped";

export type NodeState = {
  status: NodeStatus;
  message?: string;
  progress?: number;
};

export const emptyGraph = (): Graph => ({ version: 1, nodes: [], edges: [] });
