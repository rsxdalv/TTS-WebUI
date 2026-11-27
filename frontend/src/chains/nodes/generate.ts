import { asAudio, gradio, postJson } from "../api";
import { registerNode } from "../registry";
import type { NodeDef, Params } from "../types";

/** The seed a node sends: a fixed one, or a new one each run. */
const seedFor = (p: Params) =>
  p.use_random_seed ? Math.floor(Math.random() * 2 ** 31) : Number(p.seed);

const seedParams: NodeDef["params"] = [
  { key: "use_random_seed", label: "Random seed", kind: "toggle", default: true },
  { key: "seed", label: "Seed", kind: "number", default: 0, min: 0, step: 1 },
];

/** Random seeds make a run different each time, so it is not cached. */
const cacheableWithSeed = (p: Params) => !p.use_random_seed;

const text = (inputs: Record<string, unknown>, p: Params) =>
  String(inputs.text ?? p.text ?? "");

export const mmsNode: NodeDef = {
  type: "tts.mms",
  label: "MMS TTS",
  category: "Generate",
  lane: "gradio",
  inputs: { text: { type: "text", label: "Text" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "text", label: "Text", kind: "textarea", default: "" },
    { key: "language", label: "Language code", kind: "text", default: "eng" },
    { key: "speaking_rate", label: "Speaking rate", kind: "slider", min: 0.5, max: 2, step: 0.05, default: 1 },
    { key: "noise_scale", label: "Noise scale", kind: "slider", min: 0, max: 2, step: 0.01, default: 0.667 },
    { key: "noise_scale_duration", label: "Noise duration", kind: "slider", min: 0, max: 2, step: 0.01, default: 0.8 },
    ...seedParams,
  ],
  cacheable: cacheableWithSeed,
  run: async ({ signal, progress }, inputs, p) => {
    const t = text(inputs, p);
    if (!t.trim()) throw new Error("No text");
    progress(0.1, "Generating");
    const r = await gradio<{ audio: any }>(
      "mms",
      {
        text: t,
        language: p.language,
        speaking_rate: p.speaking_rate,
        noise_scale: p.noise_scale,
        noise_scale_duration: p.noise_scale_duration,
        seed: seedFor(p),
      },
      signal
    );
    return { audio: asAudio(r.audio, "mms") };
  },
};

const KOKORO_VOICES = [
  "af_heart", "af_bella", "af_nicole", "af_sarah", "af_sky",
  "am_adam", "am_michael", "bf_emma", "bf_isabella", "bm_george", "bm_lewis",
];

export const kokoroNode: NodeDef = {
  type: "tts.kokoro",
  label: "Kokoro TTS",
  category: "Generate",
  lane: "gradio",
  inputs: { text: { type: "text", label: "Text" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "text", label: "Text", kind: "textarea", default: "" },
    { key: "voice", label: "Voice", kind: "select", options: KOKORO_VOICES, default: "af_heart" },
    { key: "speed", label: "Speed", kind: "slider", min: 0.5, max: 2, step: 0.05, default: 1 },
    { key: "use_gpu", label: "Use GPU", kind: "toggle", default: true },
    ...seedParams,
  ],
  cacheable: cacheableWithSeed,
  run: async ({ signal, progress }, inputs, p) => {
    const t = text(inputs, p);
    if (!t.trim()) throw new Error("No text");
    progress(0.1, "Generating");
    const r = await gradio<{ audio: any }>(
      "kokoro_generate",
      {
        text: t,
        voice: p.voice,
        speed: p.speed,
        model_name: "hexgrad/Kokoro-82M",
        use_gpu: p.use_gpu,
        seed: seedFor(p),
      },
      signal
    );
    return { audio: asAudio(r.audio, "kokoro") };
  },
};

/**
 * An OpenAI-style endpoint saved in the Go core. The key stays there: the node
 * only names the endpoint. The clip is saved in outputs/ by Go.
 */
export const openaiNode: NodeDef = {
  type: "tts.openai",
  label: "OpenAI-style TTS",
  category: "Generate",
  lane: "openai",
  inputs: { text: { type: "text", label: "Text" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "text", label: "Text", kind: "textarea", default: "" },
    { key: "endpoint", label: "Endpoint name", kind: "text", default: "" },
    { key: "model", label: "Model", kind: "text", default: "" },
    { key: "voice", label: "Voice", kind: "text", default: "" },
    { key: "speed", label: "Speed", kind: "slider", min: 0.25, max: 4, step: 0.05, default: 1 },
  ],
  run: async ({ signal, progress }, inputs, p) => {
    const t = text(inputs, p);
    if (!t.trim()) throw new Error("No text");
    if (!String(p.endpoint).trim()) {
      throw new Error("Name an endpoint (see the OpenAI-style endpoints page)");
    }
    progress(0.1, "Generating");
    const clip = await postJson<{ wav: string }>(
      "/api/go-data/openai/speech",
      {
        endpoint: p.endpoint,
        input: t,
        model: p.model,
        voice: p.voice,
        speed: p.speed,
      },
      signal
    );
    return {
      audio: { url: `/api/webui-generations/${clip.wav}`, name: "openai-speech" },
    };
  },
};

export const registerGenerateNodes = () =>
  [mmsNode, kokoroNode, openaiNode].forEach(registerNode);
