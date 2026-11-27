import { afterEach, describe, expect, it, vi } from "vitest";
import { asAudio, postJson } from "../api";

const reply = (status: number, body: string) =>
  vi.stubGlobal("fetch", vi.fn(async () => new Response(body, { status, statusText: "Bad" })));
afterEach(() => vi.unstubAllGlobals());

describe("postJson", () => {
  const sig = new AbortController().signal;
  it("returns parsed JSON", async () => {
    reply(200, '{"a":1}');
    expect(await postJson("/x", {}, sig)).toEqual({ a: 1 });
  });
  it("uses the error field, nested or not", async () => {
    reply(500, '{"error":"model missing"}');
    await expect(postJson("/x", {}, sig)).rejects.toThrow("model missing");
    reply(500, '{"data":{"error":"oops"}}');
    await expect(postJson("/x", {}, sig)).rejects.toThrow("oops");
  });
  it("falls back to the status for empty bodies", async () => {
    reply(404, "{}");
    await expect(postJson("/x", {}, sig)).rejects.toThrow("404 Bad");
    reply(502, "");
    await expect(postJson("/x", {}, sig)).rejects.toThrow("502 Bad");
  });
  it("sends a JSON body with POST", async () => {
    const f = vi.fn(async () => new Response("{}"));
    vi.stubGlobal("fetch", f);
    await postJson("/x", { k: 1 }, sig);
    const [, init] = f.mock.calls[0] as unknown as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"k":1}');
  });

  const hang = () =>
    vi.stubGlobal(
      "fetch",
      vi.fn(
        (_u: string, init: RequestInit) =>
          new Promise((_res, rej) => {
            const fail = () => rej(new DOMException("aborted", "AbortError"));
            // like the real fetch: an aborted signal rejects at once
            if (init.signal!.aborted) fail();
            else init.signal!.addEventListener("abort", fail);
          })
      )
    );
  it("gives up after the timeout with a clear message", async () => {
    hang();
    await expect(postJson("/x", {}, sig, 20)).rejects.toThrow(/No answer/);
  });
  it("stops waiting when the caller aborts", async () => {
    hang();
    const ac = new AbortController();
    const p = postJson("/x", {}, ac.signal, 60000);
    ac.abort();
    await expect(p).rejects.toThrow(/abort/i);
  });
  it("is already stopped if the signal was aborted before the call", async () => {
    hang();
    const ac = new AbortController();
    ac.abort();
    await expect(postJson("/x", {}, ac.signal, 60000)).rejects.toThrow(/abort/i);
  });
});

describe("asAudio", () => {
  it("needs a url", () => {
    expect(() => asAudio(undefined, "x")).toThrow();
    expect(() => asAudio({}, "x")).toThrow("no audio");
    expect(asAudio({ url: "u", orig_name: "o" }, "x")).toEqual({ url: "u", name: "o" });
  });
});
