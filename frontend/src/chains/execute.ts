import { cacheKey, ResultCache } from "./cache";
import { getNodeDef } from "./registry";
import type {
  Graph,
  NodeDef,
  NodeInstance,
  NodeState,
  Value,
} from "./types";
import { validateGraph } from "./validate";

export type RunOptions = {
  signal?: AbortSignal;
  cache?: ResultCache;
  lookup?: (type: string) => NodeDef | undefined;
  onState?: (nodeId: string, state: NodeState) => void;
  onOutputs?: (nodeId: string, outputs: Record<string, Value>) => void;
};

export type RunResult = {
  outputs: Record<string, Record<string, Value>>;
  states: Record<string, NodeState>;
  /** Set when the graph was refused before any node ran. */
  problems?: string[];
};

/** A lane lets one job in at a time, in the order they ask. */
class Lane {
  private tail: Promise<unknown> = Promise.resolve();
  run<T>(job: () => Promise<T>): Promise<T> {
    const result = this.tail.then(job, job);
    this.tail = result.catch(() => undefined);
    return result;
  }
}

class Aborted extends Error {
  constructor() {
    super("Stopped");
  }
}

const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * Run a graph. Nodes whose inputs are ready run together, except that nodes of
 * one lane take turns. A failed node marks the nodes after it as skipped; the
 * other branches carry on.
 */
export const runGraph = async (
  graph: Graph,
  opts: RunOptions = {}
): Promise<RunResult> => {
  const lookup = opts.lookup ?? getNodeDef;
  const cache = opts.cache ?? new ResultCache();
  const signal = opts.signal ?? new AbortController().signal;
  const states: Record<string, NodeState> = {};
  const outputs: Record<string, Record<string, Value>> = {};

  const problems = validateGraph(graph, lookup);
  if (problems.length) {
    return { outputs, states, problems: problems.map((p) => p.message) };
  }

  const setState = (id: string, state: NodeState) => {
    states[id] = state;
    opts.onState?.(id, state);
  };
  for (const n of graph.nodes) setState(n.id, { status: "queued" });

  const lanes = new Map<string, Lane>();
  const laneFor = (name: string) => {
    if (!lanes.has(name)) lanes.set(name, new Lane());
    return lanes.get(name)!;
  };

  const promises = new Map<string, Promise<Record<string, Value> | null>>();
  const byId = new Map(graph.nodes.map((n) => [n.id, n]));

  const start = (node: NodeInstance): Promise<Record<string, Value> | null> => {
    const existing = promises.get(node.id);
    if (existing) return existing;
    const p = (async () => {
      const def = lookup(node.type)!;
      const feeding = graph.edges.filter((e) => e.to[0] === node.id);
      const upstream = await Promise.all(
        feeding.map(async (e) => ({
          edge: e,
          result: await start(byId.get(e.from[0])!),
        }))
      );
      if (upstream.some((u) => u.result === null)) {
        setState(node.id, { status: "skipped", message: "An input failed" });
        return null;
      }
      const inputs: Record<string, Value> = {};
      for (const { edge, result } of upstream) {
        const value = result![edge.from[1]];
        if (value == null) {
          setState(node.id, {
            status: "skipped",
            message: "An input has no value",
          });
          return null;
        }
        inputs[edge.to[1]] = value;
      }
      for (const [name, spec] of Object.entries(def.inputs)) {
        if (!spec.optional && !(name in inputs) && !(name in node.params)) {
          setState(node.id, {
            status: "error",
            message: `Input "${spec.label ?? name}" is not connected`,
          });
          return null;
        }
      }

      if (node.bypass) {
        const through = bypassOutputs(def, inputs);
        outputs[node.id] = through;
        opts.onOutputs?.(node.id, through);
        setState(node.id, { status: "skipped", message: "Bypassed" });
        return through;
      }

      const key = cacheKey(node.type, node.params, inputs);
      const canCache = def.cacheable ? def.cacheable(node.params) : true;
      const hit = canCache ? cache.get(node.id, key) : undefined;
      if (hit) {
        outputs[node.id] = hit;
        opts.onOutputs?.(node.id, hit);
        setState(node.id, { status: "cached" });
        return hit;
      }

      const job = async () => {
        if (signal.aborted) throw new Aborted();
        setState(node.id, { status: "running", progress: 0 });
        return def.run(
          {
            signal,
            progress: (progress, message) =>
              setState(node.id, { status: "running", progress, message }),
          },
          inputs,
          node.params
        );
      };
      try {
        const result = def.lane ? await laneFor(def.lane).run(job) : await job();
        if (signal.aborted) throw new Aborted();
        if (canCache) cache.set(node.id, key, result);
        outputs[node.id] = result;
        opts.onOutputs?.(node.id, result);
        setState(node.id, { status: "done" });
        return result;
      } catch (e) {
        setState(
          node.id,
          signal.aborted || e instanceof Aborted
            ? { status: "skipped", message: "Stopped" }
            : { status: "error", message: errorText(e) }
        );
        return null;
      }
    })();
    promises.set(node.id, p);
    return p;
  };

  await Promise.all(graph.nodes.map(start));
  return { outputs, states };
};

/** A bypassed node hands each output the first input of the same type. */
export const bypassOutputs = (
  def: NodeDef,
  inputs: Record<string, Value>
): Record<string, Value> => {
  const result: Record<string, Value> = {};
  const used = new Set<string>();
  for (const [outName, out] of Object.entries(def.outputs)) {
    const match = Object.entries(def.inputs).find(
      ([inName, spec]) =>
        spec.type === out.type && !used.has(inName) && inputs[inName] != null
    );
    if (match) {
      used.add(match[0]);
      result[outName] = inputs[match[0]];
    }
  }
  return result;
};
