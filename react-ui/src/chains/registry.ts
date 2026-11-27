import type { NodeDef, Params } from "./types";

const defs = new Map<string, NodeDef>();

/** Register a node type. Extensions call this too. A repeat type replaces it. */
export const registerNode = (def: NodeDef) => {
  defs.set(def.type, def);
};

export const getNodeDef = (type: string): NodeDef | undefined => defs.get(type);

export const listNodeDefs = (): NodeDef[] => [...defs.values()];

export const defaultParams = (def: NodeDef): Params =>
  Object.fromEntries(def.params.map((p) => [p.key, p.default]));

/** Test helper. */
export const clearRegistry = () => defs.clear();
