import { uploadFile } from "../components/FileInput";
import { encodeWav, type Pcm } from "./dsp";
import type { AudioValue } from "./types";

/** Fetch and decode a clip in the browser. */
export const loadPcm = async (url: string, signal?: AbortSignal): Promise<Pcm> => {
  const res = await fetch(url, { signal });
  if (!res.ok) throw new Error(`Could not read audio (${res.status})`);
  const bytes = await res.arrayBuffer();
  const Ctx: typeof AudioContext =
    window.AudioContext ?? (window as any).webkitAudioContext;
  const ctx = new Ctx();
  try {
    const buf = await ctx.decodeAudioData(bytes);
    return {
      sampleRate: buf.sampleRate,
      channels: Array.from({ length: buf.numberOfChannels }, (_, i) =>
        buf.getChannelData(i).slice()
      ),
    };
  } finally {
    void ctx.close();
  }
};

/** A blob URL for the browser's own players. Servers cannot read it. */
export const pcmToAudio = (pcm: Pcm, name: string): AudioValue => {
  const wav = encodeWav(pcm);
  const blob = new Blob([wav.buffer as ArrayBuffer], { type: "audio/wav" });
  return { url: URL.createObjectURL(blob), name };
};

/**
 * A URL a server-side node can read. Blob URLs (from the browser nodes) are
 * uploaded through /api/upload first; other URLs pass through.
 */
export const toServerUrl = async (
  audio: AudioValue,
  signal?: AbortSignal
): Promise<string> => {
  if (!audio.url.startsWith("blob:")) return audio.url;
  const blob = await (await fetch(audio.url, { signal })).blob();
  const stem = (audio.name ?? "chain").replace(/[^a-z0-9_-]+/gi, "_").slice(0, 40);
  const file = new File([blob], `chain-${stem}-${Date.now()}.wav`, {
    type: "audio/wav",
  });
  const uploaded = await uploadFile(file);
  if (!uploaded) throw new Error("Upload of the processed audio failed");
  return uploaded;
};
