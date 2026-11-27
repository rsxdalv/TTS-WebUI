import type { Value } from "./types";

/** FNV-1a over the UTF-16 units, 53 bits. Not secret; only to compare. */
export const hashString = (s: string): string => {
  let h1 = 0xdeadbeef ^ s.length;
  let h2 = 0x41c6ce57 ^ s.length;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    h1 = Math.imul(h1 ^ c, 2654435761);
    h2 = Math.imul(h2 ^ c, 1597334677);
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507) ^ Math.imul(h2 ^ (h2 >>> 13), 3266489909);
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507) ^ Math.imul(h1 ^ (h1 >>> 13), 3266489909);
  return (4294967296 * (2097151 & h2) + (h1 >>> 0)).toString(36);
};

/** Sorted-key JSON so the same data always gives the same text. */
export const stableJson = (v: unknown): string =>
  JSON.stringify(v, (_k, val) =>
    val && typeof val === "object" && !Array.isArray(val)
      ? Object.fromEntries(
          Object.entries(val as Record<string, unknown>).sort(([a], [b]) =>
            a < b ? -1 : a > b ? 1 : 0
          )
        )
      : val
  );

export const cacheKey = (
  type: string,
  params: unknown,
  inputs: Record<string, Value>
) => hashString(stableJson([type, params, inputs]));

export type CacheEntry = { key: string; outputs: Record<string, Value> };

/** One entry per node id. A changed key replaces the entry. */
export class ResultCache {
  private entries = new Map<string, CacheEntry>();
  get(nodeId: string, key: string) {
    const e = this.entries.get(nodeId);
    return e && e.key === key ? e.outputs : undefined;
  }
  set(nodeId: string, key: string, outputs: Record<string, Value>) {
    this.entries.set(nodeId, { key, outputs });
  }
  delete(nodeId: string) {
    this.entries.delete(nodeId);
  }
  clear() {
    this.entries.clear();
  }
}
