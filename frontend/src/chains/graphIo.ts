import type { Edge as FlowEdge, Node as FlowNode } from "@xyflow/react";
import { defaultParams, getNodeDef } from "./registry";
import type { Edge, Graph, NodeDef, NodeInstance, Params } from "./types";
import { validateGraph } from "./validate";

export const MAX_FILE_BYTES = 2 * 1024 * 1024;

export type ModuleData = {
  type: string;
  params: Params;
  bypass?: boolean;
  [key: string]: unknown;
};
export type ModuleNode = FlowNode<ModuleData, "module">;

export const toFlow = (graph: Graph): { nodes: ModuleNode[]; edges: FlowEdge[] } => ({
  nodes: graph.nodes.map((n) => ({
    id: n.id,
    type: "module",
    position: { x: n.x, y: n.y },
    data: { type: n.type, params: n.params, bypass: n.bypass },
  })),
  edges: graph.edges.map((e) => ({
    id: e.id,
    source: e.from[0],
    sourceHandle: e.from[1],
    target: e.to[0],
    targetHandle: e.to[1],
  })),
});

export const fromFlow = (nodes: ModuleNode[], edges: FlowEdge[]): Graph => ({
  version: 1,
  nodes: nodes.map(
    (n): NodeInstance => ({
      id: n.id,
      type: n.data.type,
      x: Math.round(n.position.x),
      y: Math.round(n.position.y),
      params: n.data.params,
      ...(n.data.bypass ? { bypass: true } : {}),
    })
  ),
  edges: edges
    .filter((e) => e.sourceHandle && e.targetHandle)
    .map(
      (e): Edge => ({
        id: e.id,
        from: [e.source, e.sourceHandle!],
        to: [e.target, e.targetHandle!],
      })
    ),
});

export type ParseResult =
  | { ok: true; graph: Graph }
  | { ok: false; problems: string[] };

/**
 * Read a graph from text (a file the user chose, a saved copy). The result is
 * checked and cleaned: params are limited to the node's own keys and types,
 * so a file cannot smuggle in other values. Nothing runs from here.
 */
export const parseGraph = (
  text: string,
  lookup: (t: string) => NodeDef | undefined = getNodeDef
): ParseResult => {
  if (text.length > MAX_FILE_BYTES) {
    return { ok: false, problems: ["The file is too large"] };
  }
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return { ok: false, problems: ["The file is not valid JSON"] };
  }
  const problems = validateGraph(raw, lookup);
  if (problems.length) {
    return { ok: false, problems: problems.map((p) => p.message) };
  }
  const g = raw as Graph;
  return {
    ok: true,
    graph: {
      version: 1,
      nodes: g.nodes.map((n) => ({
        id: n.id,
        type: n.type,
        x: n.x,
        y: n.y,
        params: cleanParams(lookup(n.type)!, n.params),
        ...(n.bypass === true ? { bypass: true } : {}),
      })),
      edges: g.edges.map((e) => ({ id: String(e.id), from: [e.from[0], e.from[1]], to: [e.to[0], e.to[1]] })),
    },
  };
};

/** Keep only the node's own params, with the right primitive type. */
export const cleanParams = (def: NodeDef, given: Params): Params => {
  const out = defaultParams(def);
  for (const spec of def.params) {
    const v = (given as Record<string, unknown>)[spec.key];
    if (v === undefined) continue;
    switch (spec.kind) {
      case "toggle":
        if (typeof v === "boolean") out[spec.key] = v;
        break;
      case "number":
      case "slider":
        if (typeof v === "number" && Number.isFinite(v)) out[spec.key] = v;
        break;
      case "select":
        if (typeof v === "string" && spec.options.includes(v)) out[spec.key] = v;
        break;
      default:
        if (typeof v === "string") out[spec.key] = v.slice(0, 20000);
    }
  }
  return out;
};

export const serializeGraph = (graph: Graph) => JSON.stringify(graph, null, 2);

let counter = 0;
export const newId = (prefix = "n") =>
  `${prefix}${Date.now().toString(36)}${(counter++).toString(36)}`;

/** Fill every node's params from its definition (a template leaves some out). */
export const hydrate = (
  graph: Graph,
  lookup: (t: string) => NodeDef | undefined = getNodeDef
): Graph => ({
  ...graph,
  nodes: graph.nodes.map((n) => {
    const def = lookup(n.type);
    return def ? { ...n, params: cleanParams(def, n.params) } : n;
  }),
});
