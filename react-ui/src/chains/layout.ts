import type { Graph, NodeDef } from "./types";
import { topoOrder } from "./validate";

export const NODE_WIDTH = 300;
export const COLUMN_GAP = 90;
export const ROW_GAP = 40;

/** A rough height from the rows a node shows, for spacing only. */
export const estimateHeight = (def: NodeDef | undefined) => {
  if (!def) return 120;
  const ports = Math.max(Object.keys(def.inputs).length, Object.keys(def.outputs).length);
  const params = def.params.reduce(
    (h, p) => h + (p.kind === "textarea" ? 96 : 52),
    0
  );
  return 56 + ports * 26 + params + (def.category === "Output" ? 90 : 0);
};

/**
 * Left to right: a node sits one column right of the furthest node that feeds
 * it. Within a column nodes keep their current top-to-bottom order.
 */
const CATEGORY_COLUMN: Record<string, number> = {
  Source: 0, Generate: 1, Convert: 2, Process: 3, Output: 4,
};

export const autoLayout = (
  graph: Graph,
  lookup: (t: string) => NodeDef | undefined,
  /** Heights the canvas measured, by node id. Better than the estimate. */
  heights: Record<string, number> = {}
): Record<string, { x: number; y: number }> => {
  const order = topoOrder(graph);
  if (order.length !== graph.nodes.length) return {};
  const layer = new Map<string, number>();
  const byType = new Map(graph.nodes.map((n) => [n.id, n.type]));
  for (const id of order) {
    const feeders = graph.edges.filter((e) => e.to[0] === id).map((e) => e.from[0]);
    const alone = !graph.edges.some((e) => e.from[0] === id || e.to[0] === id);
    // A node with no cables yet goes where its kind usually sits in a chain.
    const home = CATEGORY_COLUMN[lookup(byType.get(id)!)?.category ?? ""] ?? 0;
    layer.set(
      id,
      feeders.length ? Math.max(...feeders.map((f) => layer.get(f)!)) + 1 : alone ? home : 0
    );
  }
  const byNode = new Map(graph.nodes.map((n) => [n.id, n]));
  const columns = new Map<number, string[]>();
  for (const id of order) {
    const l = layer.get(id)!;
    columns.set(l, [...(columns.get(l) ?? []), id]);
  }
  const out: Record<string, { x: number; y: number }> = {};
  for (const [l, ids] of columns) {
    ids.sort((a, b) => byNode.get(a)!.y - byNode.get(b)!.y);
    let y = 0;
    for (const id of ids) {
      out[id] = { x: l * (NODE_WIDTH + COLUMN_GAP), y };
      y += (heights[id] ?? estimateHeight(lookup(byNode.get(id)!.type))) + ROW_GAP;
    }
  }
  return out;
};

/**
 * Where a new node goes: in the column of its kind (Source, Generate, ...),
 * below whatever already sits there, so nothing lands on top of anything.
 */
export const suggestPosition = (
  def: NodeDef,
  existing: { x: number; y: number; height?: number }[]
): { x: number; y: number } => {
  const x = (CATEGORY_COLUMN[def.category] ?? 0) * (NODE_WIDTH + COLUMN_GAP);
  let y = 0;
  for (const n of existing) {
    if (Math.abs(n.x - x) < NODE_WIDTH) {
      y = Math.max(y, n.y + (n.height ?? 300) + ROW_GAP);
    }
  }
  return { x, y };
};
