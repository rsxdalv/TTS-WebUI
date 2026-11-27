import { registerNode } from "../registry";
import type { NodeDef, Value } from "../types";

/** Shows what arrives. The page reads its input to draw the player. */
export const previewNode: NodeDef = {
  type: "out.preview",
  label: "Preview",
  category: "Output",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [],
  run: async (_c, i) => ({ audio: i.audio as Value }),
};

/** Two players side by side for listening tests. */
export const compareNode: NodeDef = {
  type: "out.compare",
  label: "Compare A/B",
  category: "Output",
  inputs: {
    a: { type: "audio", label: "A" },
    b: { type: "audio", label: "B" },
  },
  outputs: {},
  params: [],
  run: async () => ({}),
};

/** Shows a text or number that arrives. */
export const textViewNode: NodeDef = {
  type: "out.text",
  label: "Show text",
  category: "Output",
  inputs: { text: { type: "text", label: "Text" } },
  outputs: {},
  params: [],
  run: async () => ({}),
};

export const registerOutputNodes = () =>
  [previewNode, compareNode, textViewNode].forEach(registerNode);
