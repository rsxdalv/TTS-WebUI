/**
 * Plain sample math for the browser-side audio nodes. No Web Audio here, so
 * it runs in tests. Channels are Float32Arrays of equal length.
 */
export type Pcm = { sampleRate: number; channels: Float32Array[] };

export const frames = (p: Pcm) => p.channels[0]?.length ?? 0;
export const seconds = (p: Pcm) => frames(p) / p.sampleRate;

export const dbToGain = (db: number) => Math.pow(10, db / 20);

export const gain = (p: Pcm, db: number): Pcm => {
  const g = dbToGain(db);
  return {
    sampleRate: p.sampleRate,
    channels: p.channels.map((c) => c.map((s) => s * g)),
  };
};

/** Keep [start, end) in seconds. An end of 0 or less means "to the end". */
export const trim = (p: Pcm, start: number, end: number): Pcm => {
  const n = frames(p);
  const a = Math.min(n, Math.max(0, Math.round(start * p.sampleRate)));
  const b = end > 0 ? Math.min(n, Math.round(end * p.sampleRate)) : n;
  return {
    sampleRate: p.sampleRate,
    channels: p.channels.map((c) => c.slice(a, Math.max(a, b))),
  };
};

/** Linear fade in and out, in seconds. The fades never overlap past the middle. */
export const fade = (p: Pcm, inSec: number, outSec: number): Pcm => {
  const n = frames(p);
  const fi = Math.min(n, Math.round(Math.max(0, inSec) * p.sampleRate));
  const fo = Math.min(n - fi, Math.round(Math.max(0, outSec) * p.sampleRate));
  return {
    sampleRate: p.sampleRate,
    channels: p.channels.map((c) => {
      const o = c.slice();
      for (let i = 0; i < fi; i++) o[i] *= i / fi;
      for (let i = 0; i < fo; i++) o[n - 1 - i] *= i / fo;
      return o;
    }),
  };
};

/** Linear interpolation. Good enough for joining clips, not for mastering. */
export const resample = (p: Pcm, rate: number): Pcm => {
  if (rate === p.sampleRate) return p;
  const n = Math.max(1, Math.round((frames(p) * rate) / p.sampleRate));
  const ratio = p.sampleRate / rate;
  return {
    sampleRate: rate,
    channels: p.channels.map((c) => {
      const o = new Float32Array(n);
      for (let i = 0; i < n; i++) {
        const x = i * ratio;
        const i0 = Math.floor(x);
        const i1 = Math.min(c.length - 1, i0 + 1);
        const t = x - i0;
        o[i] = (c[Math.min(c.length - 1, i0)] ?? 0) * (1 - t) + (c[i1] ?? 0) * t;
      }
      return o;
    }),
  };
};

/** Make the channel count match: mono is copied, extra channels are dropped. */
export const withChannels = (p: Pcm, count: number): Pcm => ({
  sampleRate: p.sampleRate,
  channels: Array.from({ length: count }, (_, i) =>
    p.channels.length === 1 ? p.channels[0] : p.channels[i] ?? p.channels[0]
  ),
});

const align = (a: Pcm, b: Pcm): [Pcm, Pcm] => {
  const rate = Math.max(a.sampleRate, b.sampleRate);
  const ch = Math.max(a.channels.length, b.channels.length);
  return [
    withChannels(resample(a, rate), ch),
    withChannels(resample(b, rate), ch),
  ];
};

/** A, then silence of gapSec, then B. */
export const concat = (a0: Pcm, b0: Pcm, gapSec: number): Pcm => {
  const [a, b] = align(a0, b0);
  const gap = Math.max(0, Math.round(gapSec * a.sampleRate));
  return {
    sampleRate: a.sampleRate,
    channels: a.channels.map((c, i) => {
      const o = new Float32Array(c.length + gap + b.channels[i].length);
      o.set(c, 0);
      o.set(b.channels[i], c.length + gap);
      return o;
    }),
  };
};

/** Add B onto A (B starts offsetSec in). Length is the longer of the two. */
export const mix = (a0: Pcm, b0: Pcm, levelDb: number, offsetSec: number): Pcm => {
  const [a, b] = align(a0, b0);
  const g = dbToGain(levelDb);
  const off = Math.max(0, Math.round(offsetSec * a.sampleRate));
  const len = Math.max(frames(a), off + frames(b));
  return {
    sampleRate: a.sampleRate,
    channels: a.channels.map((c, i) => {
      const o = new Float32Array(len);
      o.set(c, 0);
      const bc = b.channels[i];
      for (let k = 0; k < bc.length; k++) o[off + k] += bc[k] * g;
      return o;
    }),
  };
};

export const peak = (p: Pcm) => {
  let m = 0;
  for (const c of p.channels) for (const s of c) m = Math.max(m, Math.abs(s));
  return m;
};

/** 16-bit PCM WAV. Samples are clipped to [-1, 1]. */
export const encodeWav = (p: Pcm): Uint8Array => {
  const ch = p.channels.length;
  const n = frames(p);
  const dataBytes = n * ch * 2;
  const out = new Uint8Array(44 + dataBytes);
  const v = new DataView(out.buffer);
  const str = (o: number, s: string) =>
    [...s].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)));
  str(0, "RIFF");
  v.setUint32(4, 36 + dataBytes, true);
  str(8, "WAVE");
  str(12, "fmt ");
  v.setUint32(16, 16, true);
  v.setUint16(20, 1, true);
  v.setUint16(22, ch, true);
  v.setUint32(24, p.sampleRate, true);
  v.setUint32(28, p.sampleRate * ch * 2, true);
  v.setUint16(32, ch * 2, true);
  v.setUint16(34, 16, true);
  str(36, "data");
  v.setUint32(40, dataBytes, true);
  let o = 44;
  for (let i = 0; i < n; i++) {
    for (let c = 0; c < ch; c++) {
      const s = Math.max(-1, Math.min(1, p.channels[c][i]));
      v.setInt16(o, s < 0 ? s * 0x8000 : s * 0x7fff, true);
      o += 2;
    }
  }
  return out;
};
