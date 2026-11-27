import type { Graph, NodeDef, ValueType } from "./types";

export const MAX_NODES = 500;
export const MAX_EDGES = 2000;

type Lookup = (type: string) => NodeDef | undefined;

/** Which output types may feed which input types. Numbers may become text. */
export const canConnect = (from: ValueType, to: ValueType) =>
  from === to || (from === "number" && to === "text");

export type Problem = { nodeId?: string; edgeId?: string; message: string };

/**
 * Check a graph from outside (an import, a saved file) before it is shown or
 * run. It never throws: the problems come back as a list.
 */
export const validateGraph = (graph: unknown, lookup: Lookup): Problem[] => {
  const problems: Problem[] = [];
  const bad = (message: string, extra: Partial<Problem> = {}) =>
    problems.push({ message, ...extra });

  if (!graph || typeof graph !== "object") {
    return [{ message: "The graph is not an object" }];
  }
  const g = graph as Partial<Graph>;
  if (g.version !== 1) bad("Unknown graph version");
  if (!Array.isArray(g.nodes) || !Array.isArray(g.edges)) {
    bad("The graph needs a nodes list and an edges list");
    return problems;
  }
  if (g.nodes.length > MAX_NODES) bad(`More than ${MAX_NODES} nodes`);
  if (g.edges.length > MAX_EDGES) bad(`More than ${MAX_EDGES} edges`);
  if (problems.length) return problems;

  const byId = new Map<string, NodeDef | undefined>();
  for (const n of g.nodes) {
    if (!n || typeof n.id !== "string" || typeof n.type !== "string") {
      bad("A node needs a string id and type");
      continue;
    }
    if (byId.has(n.id)) bad("Duplicate node id", { nodeId: n.id });
    const def = lookup(n.type);
    byId.set(n.id, def);
    if (!def) bad(`Unknown node type "${n.type}"`, { nodeId: n.id });
    if (!Number.isFinite(n.x) || !Number.isFinite(n.y)) {
      bad("A node position is not a number", { nodeId: n.id });
    }
    if (!n.params || typeof n.params !== "object") {
      bad("A node needs params", { nodeId: n.id });
    }
  }

  const fed = new Set<string>();
  for (const e of g.edges) {
    const where = { edgeId: e?.id };
    if (!e || !Array.isArray(e.from) || !Array.isArray(e.to)) {
      bad("An edge needs from and to", where);
      continue;
    }
    const [fromId, fromPort] = e.from;
    const [toId, toPort] = e.to;
    if (!byId.has(fromId) || !byId.has(toId)) {
      bad("An edge points at a missing node", where);
      continue;
    }
    const a = byId.get(fromId)?.outputs[fromPort];
    const b = byId.get(toId)?.inputs[toPort];
    if (!byId.get(fromId) || !byId.get(toId)) continue; // already reported
    if (!a) bad(`No output "${fromPort}" on that node`, where);
    if (!b) bad(`No input "${toPort}" on that node`, where);
    if (a && b && !canConnect(a.type, b.type)) {
      bad(`Cannot connect ${a.type} to ${b.type}`, where);
    }
    const key = `${toId}:${toPort}`;
    if (fed.has(key)) bad("An input has more than one cable", where);
    fed.add(key);
  }
  if (problems.length) return problems;

  if (hasCycle(g as Graph)) bad("The graph has a cycle");
  return problems;
};

/** Kahn's algorithm. */
export const hasCycle = (graph: Graph): boolean =>
  topoOrder(graph).length !== graph.nodes.length;

/** Node ids in an order where every node follows the nodes that feed it. */
export const topoOrder = (graph: Graph): string[] => {
  const indegree = new Map(graph.nodes.map((n) => [n.id, 0]));
  const out = new Map<string, string[]>();
  for (const e of graph.edges) {
    indegree.set(e.to[0], (indegree.get(e.to[0]) ?? 0) + 1);
    out.set(e.from[0], [...(out.get(e.from[0]) ?? []), e.to[0]]);
  }
  const queue = [...indegree].filter(([, d]) => d === 0).map(([id]) => id);
  const order: string[] = [];
  while (queue.length) {
    const id = queue.shift()!;
    order.push(id);
    for (const next of out.get(id) ?? []) {
      const d = (indegree.get(next) ?? 0) - 1;
      indegree.set(next, d);
      if (d === 0) queue.push(next);
    }
  }
  return order;
};
