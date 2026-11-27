import { asAudio, gradio } from "../api";
import { toServerUrl } from "../audioIo";
import { registerNode } from "../registry";
import type { NodeDef, AudioValue } from "../types";

export const rvcNode: NodeDef = {
  type: "convert.rvc",
  label: "RVC voice",
  category: "Convert",
  lane: "gradio",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: { audio: { type: "audio", label: "Audio" } },
  params: [
    { key: "model", label: "Model file", kind: "text", default: "" },
    { key: "index", label: "Index file", kind: "text", default: "" },
    { key: "pitch_up_key", label: "Pitch (semitones)", kind: "number", default: 0, step: 1 },
    { key: "pitch_collection_method", label: "Pitch method", kind: "select", options: ["harvest", "reaper", "melodia"], default: "harvest" },
    { key: "search_feature_ratio", label: "Search feature ratio", kind: "slider", min: 0, max: 1, step: 0.01, default: 0.66 },
    { key: "filter_radius_pitch", label: "Filter radius", kind: "slider", min: 0, max: 10, step: 1, default: 3 },
    { key: "voice_envelope_normalizaiton", label: "Envelope mix", kind: "slider", min: 0, max: 1, step: 0.01, default: 0.5 },
    { key: "protect_breath_sounds", label: "Protect breaths", kind: "slider", min: 0, max: 0.5, step: 0.01, default: 0.33 },
    { key: "device", label: "Device", kind: "select", options: ["cuda:0", "cpu", "mps"], default: "cuda:0" },
    { key: "use_half_precision_model", label: "Half precision", kind: "toggle", default: false },
  ],
  run: async ({ signal, progress }, inputs, p) => {
    if (!String(p.model).trim()) throw new Error("Choose an RVC model file");
    progress(0.1, "Converting");
    const original_audio = await toServerUrl(inputs.audio as AudioValue, signal);
    const r = await gradio<{ audio: any }>(
      "rvc",
      {
        ...p,
        pitch_up_key: String(p.pitch_up_key),
        resample_sample_rate: 0,
        original_audio,
      },
      signal
    );
    return { audio: asAudio(r.audio, "rvc") };
  },
};

export const demucsNode: NodeDef = {
  type: "convert.demucs",
  label: "Demucs stems",
  category: "Convert",
  lane: "gradio",
  inputs: { audio: { type: "audio", label: "Audio" } },
  outputs: {
    drums: { type: "audio", label: "Drums" },
    bass: { type: "audio", label: "Bass" },
    other: { type: "audio", label: "Other" },
    vocals: { type: "audio", label: "Vocals" },
  },
  params: [],
  run: async ({ signal, progress }, inputs) => {
    progress(0.1, "Separating");
    const path = await toServerUrl(inputs.audio as AudioValue, signal);
    const r = await gradio<any[]>(
      "demucs",
      { audio: { __type: "local_cache_file", path } },
      signal
    );
    const names = ["drums", "bass", "other", "vocals"] as const;
    if (!Array.isArray(r) || r.length < names.length) {
      throw new Error("Demucs returned fewer than four stems");
    }
    return Object.fromEntries(names.map((n, i) => [n, asAudio(r[i], n)]));
  },
};

export const registerConvertNodes = () =>
  [rvcNode, demucsNode].forEach(registerNode);
