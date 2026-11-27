import { describe, expect, it } from "vitest";
import {
  concat, dbToGain, encodeWav, fade, frames, gain, mix, peak, resample, seconds, trim, withChannels,
  type Pcm,
} from "../dsp";

const mono = (vals: number[], rate = 4): Pcm => ({
  sampleRate: rate,
  channels: [Float32Array.from(vals)],
});

describe("dsp", () => {
  it("gain: 0 dB is identity, -6.02 dB halves", () => {
    expect(gain(mono([1, -1]), 0).channels[0]).toEqual(Float32Array.from([1, -1]));
    expect(dbToGain(-6.0206)).toBeCloseTo(0.5, 4);
    expect(gain(mono([1]), 20).channels[0][0]).toBeCloseTo(10, 5);
  });

  it("trim: keeps the range, end 0 means to the end, clamps", () => {
    const p = mono([1, 2, 3, 4, 5, 6, 7, 8]); // 2 s at 4 Hz
    expect([...trim(p, 0.5, 1.5).channels[0]]).toEqual([3, 4, 5, 6]);
    expect([...trim(p, 1, 0).channels[0]]).toEqual([5, 6, 7, 8]);
    expect(frames(trim(p, 5, 9))).toBe(0);
    expect(frames(trim(p, -3, 99))).toBe(8);
    expect(frames(trim(p, 1.5, 0.5))).toBe(0);
  });

  it("fade: in starts at 0, out ends at 0, middle untouched", () => {
    const p = mono(new Array(8).fill(1));
    const f = fade(p, 0.5, 0.5).channels[0]; // 2 frames each
    expect(f[0]).toBe(0);
    expect(f[1]).toBeCloseTo(0.5);
    expect(f[7]).toBe(0);
    expect(f[4]).toBe(1);
    // overlong fades never break
    expect(() => fade(p, 99, 99)).not.toThrow();
    expect(frames(fade(p, 99, 99))).toBe(8);
  });

  it("resample: length scales, same rate is identity, endpoints hold", () => {
    const p = mono([0, 1, 2, 3], 4);
    expect(resample(p, 4)).toBe(p);
    const up = resample(p, 8);
    expect(up.sampleRate).toBe(8);
    expect(frames(up)).toBe(8);
    expect(up.channels[0][0]).toBe(0);
    expect(up.channels[0][2]).toBeCloseTo(1);
    expect(frames(resample(mono([]), 8))).toBe(1);
  });

  it("withChannels: mono is copied to stereo", () => {
    const s = withChannels(mono([1, 2]), 2);
    expect(s.channels.length).toBe(2);
    expect(s.channels[1]).toEqual(s.channels[0]);
  });

  it("concat: joins with a gap, and matches rate and channels", () => {
    const a = mono([1, 1], 4);
    const b = mono([2, 2], 8);
    const c = concat(a, b, 0.25);
    expect(c.sampleRate).toBe(8);
    expect(seconds(c)).toBeCloseTo(0.5 + 0.25 + 0.25, 5);
    const st = concat({ sampleRate: 4, channels: [Float32Array.of(1), Float32Array.of(2)] }, mono([3]), 0);
    expect(st.channels.length).toBe(2);
    expect(st.channels[1][1]).toBe(3);
  });

  it("mix: adds with level and offset, length is the longer end", () => {
    const a = mono([1, 1, 1, 1]);
    const b = mono([1, 1]);
    const m = mix(a, b, 0, 0.5).channels[0]; // offset 2 frames
    expect([...m]).toEqual([1, 1, 2, 2]);
    const long = mix(a, b, -6.0206, 1.5).channels[0];
    expect(long.length).toBe(8);
    expect(long[6]).toBeCloseTo(0.5, 3);
  });

  it("encodeWav: header, size and sample values", () => {
    const p: Pcm = { sampleRate: 8000, channels: [Float32Array.of(0, 1, -1, 2, -2)] };
    const w = encodeWav(p);
    const v = new DataView(w.buffer);
    const text = (o: number) => String.fromCharCode(...w.slice(o, o + 4));
    expect(text(0)).toBe("RIFF");
    expect(text(8)).toBe("WAVE");
    expect(w.length).toBe(44 + 10);
    expect(v.getUint32(24, true)).toBe(8000);
    expect(v.getUint16(22, true)).toBe(1);
    expect(v.getInt16(46, true)).toBe(32767);
    expect(v.getInt16(48, true)).toBe(-32768);
    expect(v.getInt16(50, true)).toBe(32767); // clipped
    expect(v.getInt16(52, true)).toBe(-32768);
  });

  it("encodeWav: stereo is interleaved", () => {
    const w = encodeWav({ sampleRate: 4, channels: [Float32Array.of(1), Float32Array.of(-1)] });
    const v = new DataView(w.buffer);
    expect(v.getUint16(22, true)).toBe(2);
    expect(v.getInt16(44, true)).toBe(32767);
    expect(v.getInt16(46, true)).toBe(-32768);
  });

  it("peak", () => {
    expect(peak(mono([0.2, -0.7, 0.5]))).toBeCloseTo(0.7);
  });
});
