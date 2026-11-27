import { registerNode } from "../registry";
import type { NodeDef } from "../types";

export const textNode: NodeDef = {
  type: "source.text",
  label: "Text",
  category: "Source",
  inputs: {},
  outputs: { text: { type: "text", label: "Text" } },
  params: [
    {
      key: "text",
      label: "Text",
      kind: "textarea",
      default: "Hello! This is a test of the chain editor.",
    },
  ],
  run: async (_c, _i, p) => ({ text: String(p.text) }),
};

export const numberNode: NodeDef = {
  type: "source.number",
  label: "Number",
  category: "Source",
  inputs: {},
  outputs: { value: { type: "number", label: "Value" } },
  params: [{ key: "value", label: "Value", kind: "number", default: 0 }],
  run: async (_c, _i, p) => ({ value: Number(p.value) }),
};

/** A clip by URL or path the browser can read: /file-input-cache/x.wav, /api/webui-generations/... */
export const audioUrlNode: NodeDef = {
  type: "source.audio",
  label: "Audio file",
  category: "Source",
  inputs: {},
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    {
      key: "url",
      label: "URL or path",
      kind: "text",
      default: "",
    },
  ],
  run: async (_c, _i, p) => {
    const url = String(p.url).trim();
    if (!url) throw new Error("Enter an audio URL or path");
    if (/^javascript:/i.test(url)) throw new Error("Not an audio URL");
    return { audio: { url, name: url.split("/").pop() || "audio" } };
  },
};

export const registerSourceNodes = () =>
  [textNode, numberNode, audioUrlNode].forEach(registerNode);
