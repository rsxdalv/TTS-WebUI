import type { Graph } from "./types";

const n = (id: string, type: string, x: number, y: number, params = {}) => ({
  id, type, x, y, params,
});
const e = (id: string, a: string, ap: string, b: string, bp: string) => ({
  id, from: [a, ap] as [string, string], to: [b, bp] as [string, string],
});

/** Starting points. Params left out take the node's defaults when loaded. */
export const templates: { name: string; description: string; graph: Graph }[] = [
  {
    name: "Speak and listen",
    description: "Text to speech with MMS, then a fade, then a preview. Runs on a CPU.",
    graph: {
      version: 1,
      nodes: [
        n("t", "source.text", 0, 0),
        n("s", "tts.mms", 390, 0),
        n("f", "fx.fade", 780, 0),
        n("p", "out.preview", 1170, 0),
      ],
      edges: [
        e("e1", "t", "text", "s", "text"),
        e("e2", "s", "audio", "f", "audio"),
        e("e3", "f", "audio", "p", "audio"),
      ],
    },
  },
  {
    name: "Compare two voices",
    description: "One text through two engines, listened to side by side.",
    graph: {
      version: 1,
      nodes: [
        n("t", "source.text", 0, 120),
        n("k", "tts.kokoro", 390, 0),
        n("m", "tts.mms", 390, 420),
        n("c", "out.compare", 780, 150),
      ],
      edges: [
        e("e1", "t", "text", "k", "text"),
        e("e2", "t", "text", "m", "text"),
        e("e3", "k", "audio", "c", "a"),
        e("e4", "m", "audio", "c", "b"),
      ],
    },
  },
  {
    name: "Speak, convert, save",
    description: "Text to speech, then an RVC voice. Pick an RVC model in the node.",
    graph: {
      version: 1,
      nodes: [
        n("t", "source.text", 0, 0),
        n("s", "tts.kokoro", 390, 0),
        n("r", "convert.rvc", 780, 0),
        n("p", "out.preview", 1170, 0),
      ],
      edges: [
        e("e1", "t", "text", "s", "text"),
        e("e2", "s", "audio", "r", "audio"),
        e("e3", "r", "audio", "p", "audio"),
      ],
    },
  },
];
