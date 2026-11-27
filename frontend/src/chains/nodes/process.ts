import { loadPcm, pcmToAudio } from "../audioIo";
import { concat, fade, gain, mix, trim } from "../dsp";
import { registerNode } from "../registry";
import type { AudioValue, NodeDef } from "../types";

const load = (v: unknown, signal: AbortSignal) =>
  loadPcm((v as AudioValue).url, signal);

export const gainNode: NodeDef = {
  type: "fx.gain",
  label: "Gain",
  category: "Process",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [{ key: "db", label: "Gain (dB)", kind: "slider", min: -36, max: 24, step: 0.5, default: 0 }],
  run: async ({ signal }, i, p) => ({
    audio: pcmToAudio(gain(await load(i.audio, signal), Number(p.db)), "gain"),
  }),
};

export const trimNode: NodeDef = {
  type: "fx.trim",
  label: "Trim",
  category: "Process",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "start", label: "Start (s)", kind: "number", min: 0, step: 0.1, default: 0 },
    { key: "end", label: "End (s, 0 = end)", kind: "number", min: 0, step: 0.1, default: 0 },
  ],
  run: async ({ signal }, i, p) => ({
    audio: pcmToAudio(trim(await load(i.audio, signal), Number(p.start), Number(p.end)), "trim"),
  }),
};

export const fadeNode: NodeDef = {
  type: "fx.fade",
  label: "Fade",
  category: "Process",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "fade_in", label: "Fade in (s)", kind: "number", min: 0, step: 0.05, default: 0.05 },
    { key: "fade_out", label: "Fade out (s)", kind: "number", min: 0, step: 0.05, default: 0.2 },
  ],
  run: async ({ signal }, i, p) => ({
    audio: pcmToAudio(fade(await load(i.audio, signal), Number(p.fade_in), Number(p.fade_out)), "fade"),
  }),
};

export const concatNode: NodeDef = {
  type: "fx.concat",
  label: "Join",
  category: "Process",
  inputs: {
    a: { type: "audio", label: "First" },
    b: { type: "audio", label: "Second" },
  },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [{ key: "gap", label: "Gap (s)", kind: "number", min: 0, step: 0.1, default: 0.3 }],
  run: async ({ signal }, i, p) => {
    const [a, b] = await Promise.all([load(i.a, signal), load(i.b, signal)]);
    return { audio: pcmToAudio(concat(a, b, Number(p.gap)), "join") };
  },
};

export const mixNode: NodeDef = {
  type: "fx.mix",
  label: "Mix",
  category: "Process",
  inputs: {
    a: { type: "audio", label: "Main" },
    b: { type: "audio", label: "Layer" },
  },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "level", label: "Layer level (dB)", kind: "slider", min: -36, max: 12, step: 0.5, default: -6 },
    { key: "offset", label: "Layer offset (s)", kind: "number", min: 0, step: 0.1, default: 0 },
  ],
  run: async ({ signal }, i, p) => {
    const [a, b] = await Promise.all([load(i.a, signal), load(i.b, signal)]);
    return { audio: pcmToAudio(mix(a, b, Number(p.level), Number(p.offset)), "mix") };
  },
};

export const registerProcessNodes = () =>
  [gainNode, trimNode, fadeNode, concatNode, mixNode].forEach(registerNode);
