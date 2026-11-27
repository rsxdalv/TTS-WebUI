/**
 * Simple Remixer logic, in the browser (no backend). Same rule as
 * the retired Gradio extension_simple_remixer: the slots of one column are mixed (summed, the
 * shorter ones padded with silence), then the columns are joined one after the
 * other. Empty slots are skipped; nothing to join gives null.
 *
 * Every slot is mono at one sample rate: the page decodes with a single
 * AudioContext, which converts the rate and the channels. (The extension
 * resampled to the highest rate of its slots; here that is the context's rate.)
 */

export type RemixSlot = { samples: Float32Array } | null;

/** Sums the non-empty slots of one column. Null when all are empty. */
export const mixColumn = (column: RemixSlot[]): Float32Array | null => {
  const used = column.filter((s): s is { samples: Float32Array } => s !== null);
  if (!used.length) return null;
  const length = Math.max(...used.map((s) => s.samples.length));
  const out = new Float32Array(length);
  for (const { samples } of used) {
    for (let i = 0; i < samples.length; i++) out[i] += samples[i];
  }
  return out;
};

/** Mixes each column, then joins the columns in order. */
export const remix = (columns: RemixSlot[][]): Float32Array | null => {
  const parts = columns
    .map(mixColumn)
    .filter((p): p is Float32Array => p !== null);
  if (!parts.length) return null;
  const out = new Float32Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  for (const p of parts) {
    out.set(p, offset);
    offset += p.length;
  }
  return out;
};

/** 16-bit PCM mono WAV. Samples are clipped to [-1, 1] (a sum can exceed it). */
export const encodeWav = (samples: Float32Array, sampleRate: number): Blob => {
  const buffer = new ArrayBuffer(44 + samples.length * 2);
  const view = new DataView(buffer);
  const text = (offset: number, s: string) => {
    for (let i = 0; i < s.length; i++) view.setUint8(offset + i, s.charCodeAt(i));
  };
  text(0, "RIFF");
  view.setUint32(4, 36 + samples.length * 2, true);
  text(8, "WAVE");
  text(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true); // PCM
  view.setUint16(22, 1, true); // mono
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  text(36, "data");
  view.setUint32(40, samples.length * 2, true);
  for (let i = 0; i < samples.length; i++) {
    const s = Math.max(-1, Math.min(1, samples[i]));
    view.setInt16(44 + i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  return new Blob([buffer], { type: "audio/wav" });
};

/** Decodes a file to mono samples at the context's sample rate. */
export const decodeToMono = async (
  context: AudioContext,
  file: Blob
): Promise<Float32Array> => {
  const audio = await context.decodeAudioData(await file.arrayBuffer());
  const out = new Float32Array(audio.length);
  for (let c = 0; c < audio.numberOfChannels; c++) {
    const data = audio.getChannelData(c);
    for (let i = 0; i < out.length; i++) out[i] += data[i] / audio.numberOfChannels;
  }
  return out;
};
