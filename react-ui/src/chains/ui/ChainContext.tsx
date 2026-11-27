import React from "react";
import type { NodeState, Params, Value } from "../types";

export type ChainContextValue = {
  states: Record<string, NodeState>;
  outputs: Record<string, Record<string, Value>>;
  /** "nodeId:port" of every input that has a cable. */
  connected: Set<string>;
  /** The value that arrived on an input, from the node that feeds it. */
  valueAt: (nodeId: string, port: string) => Value;
  setParam: (nodeId: string, key: string, value: Params[string]) => void;
  toggleBypass: (nodeId: string) => void;
  removeNode: (nodeId: string) => void;
  running: boolean;
};

export const ChainContext = React.createContext<ChainContextValue | null>(null);

export const useChain = () => {
  const ctx = React.useContext(ChainContext);
  if (!ctx) throw new Error("useChain outside the chain editor");
  return ctx;
};
