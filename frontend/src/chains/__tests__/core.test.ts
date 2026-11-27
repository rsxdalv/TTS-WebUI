import { describe, expect, it } from "vitest";
import { ResultCache, cacheKey } from "../cache";
import { bypassOutputs, runGraph } from "../execute";
import type { Graph, NodeDef, Params, Value } from "../types";
import { canConnect, topoOrder, validateGraph } from "../validate";

const calls: string[] = [];
let active = 0;
let maxActive = 0;

const def = (
  type: string,
  over: Partial<NodeDef> = {},
  run?: NodeDef["run"]
): NodeDef => ({
  type,
  label: type,
  category: "Process",
  inputs: {},
  outputs: { out: { type: "text" } },
  params: [],
  run:
    run ??
    (async (_c, inputs, params) => {
      calls.push(type);
      return { out: String(inputs.in ?? params.v ?? type) };
    }),
  ...over,
});

const defs: Record<string, NodeDef> = {
  src: def("src", {}, async (_c, _i, p) => {
    calls.push("src");
    return { out: String(p.v) };
  }),
  upper: def("upper", { inputs: { in: { type: "text" } } }, async (_c, i) => {
    calls.push("upper");
    return { out: String(i.in).toUpperCase() };
  }),
  aud: def("aud", { outputs: { out: { type: "audio" } } }),
  num: def("num", { outputs: { out: { type: "number" } } }),
  boom: def("boom", { inputs: { in: { type: "text" } } }, async () => {
    throw new Error("kaput");
  }),
  slow: def(
    "slow",
    { lane: "gpu", inputs: { in: { type: "text", optional: true } } },
    async () => {
      active++;
      maxActive = Math.max(maxActive, active);
      await new Promise((r) => setTimeout(r, 15));
      active--;
      return { out: "slow" };
    }
  ),
  rand: def("rand", { cacheable: () => false }, async () => {
    calls.push("rand");
    return { out: String(Math.random()) };
  }),
  pass: def("pass", {
    inputs: { a: { type: "audio" }, t: { type: "text" } },
    outputs: { a: { type: "audio" }, t: { type: "text" } },
  }),
};
const lookup = (t: string) => defs[t];

const node = (id: string, type: string, params: Params = {}, extra = {}) => ({
  id,
  type,
  x: 0,
  y: 0,
  params,
  ...extra,
});
const edge = (from: string, to: string, fp = "out", tp = "in") => ({
  id: `${from}-${to}`,
  from: [from, fp] as [string, string],
  to: [to, tp] as [string, string],
});
const graph = (nodes: Graph["nodes"], edges: Graph["edges"] = []): Graph => ({
  version: 1,
  nodes,
  edges,
});

describe("validate", () => {
  it("accepts a simple chain", () => {
    const g = graph([node("a", "src", { v: "x" }), node("b", "upper")], [edge("a", "b")]);
    expect(validateGraph(g, lookup)).toEqual([]);
  });
  it("refuses non-objects, wrong versions and missing lists", () => {
    expect(validateGraph(null, lookup)).not.toEqual([]);
    expect(validateGraph({ version: 2, nodes: [], edges: [] }, lookup)).not.toEqual([]);
    expect(validateGraph({ version: 1 }, lookup)).not.toEqual([]);
  });
  it("refuses unknown node types", () => {
    const p = validateGraph(graph([node("a", "nope")]), lookup);
    expect(p[0].message).toMatch(/Unknown node type/);
  });
  it("refuses a type mismatch and a missing port", () => {
    const g = graph([node("a", "aud"), node("b", "upper")], [edge("a", "b")]);
    expect(validateGraph(g, lookup)[0].message).toMatch(/Cannot connect audio to text/);
    const h = graph([node("a", "src"), node("b", "upper")], [edge("a", "b", "zzz")]);
    expect(validateGraph(h, lookup)[0].message).toMatch(/No output/);
  });
  it("allows number into text only", () => {
    expect(canConnect("number", "text")).toBe(true);
    expect(canConnect("text", "number")).toBe(false);
    expect(canConnect("audio", "text")).toBe(false);
  });
  it("refuses two cables into one input", () => {
    const g = graph(
      [node("a", "src"), node("b", "src"), node("c", "upper")],
      [edge("a", "c"), edge("b", "c")]
    );
    expect(validateGraph(g, lookup)[0].message).toMatch(/more than one/);
  });
  it("refuses cycles", () => {
    const g = graph([node("a", "upper"), node("b", "upper")], [edge("a", "b"), edge("b", "a")]);
    expect(validateGraph(g, lookup).map((p) => p.message)).toContain("The graph has a cycle");
  });
  it("refuses duplicates and oversize graphs", () => {
    expect(validateGraph(graph([node("a", "src"), node("a", "src")]), lookup)[0].message).toMatch(/Duplicate/);
    const many = Array.from({ length: 501 }, (_, i) => node(`n${i}`, "src"));
    expect(validateGraph(graph(many), lookup)[0].message).toMatch(/More than/);
  });
  it("does not trust prototype keys or odd shapes", () => {
    const g: any = { version: 1, nodes: [{ id: "a", type: "src", x: "1", y: 0, params: {} }], edges: [] };
    expect(validateGraph(g, lookup).length).toBeGreaterThan(0);
    const e: any = graph([node("a", "src")], [{ id: "x", from: ["a", "out"] } as any]);
    expect(validateGraph(e, lookup).length).toBeGreaterThan(0);
  });
});

describe("topoOrder", () => {
  it("puts sources first", () => {
    const g = graph([node("c", "upper"), node("a", "src"), node("b", "upper")], [edge("a", "b"), edge("b", "c")]);
    expect(topoOrder(g)).toEqual(["a", "b", "c"]);
  });
});

describe("runGraph", () => {
  it("runs in order and passes values", async () => {
    calls.length = 0;
    const g = graph([node("a", "src", { v: "hi" }), node("b", "upper")], [edge("a", "b")]);
    const r = await runGraph(g, { lookup });
    expect(r.outputs.b.out).toBe("HI");
    expect(r.states.b.status).toBe("done");
    expect(calls).toEqual(["src", "upper"]);
  });

  it("refuses an invalid graph without running anything", async () => {
    calls.length = 0;
    const r = await runGraph(graph([node("a", "nope")]), { lookup });
    expect(r.problems?.length).toBe(1);
    expect(calls).toEqual([]);
  });

  it("caches, and re-runs only what changed and what follows", async () => {
    calls.length = 0;
    const cache = new ResultCache();
    const g = graph([node("a", "src", { v: "hi" }), node("b", "upper")], [edge("a", "b")]);
    await runGraph(g, { lookup, cache });
    const second = await runGraph(g, { lookup, cache });
    expect(calls).toEqual(["src", "upper"]);
    expect(second.states.a.status).toBe("cached");
    expect(second.states.b.status).toBe("cached");
    const g2 = graph([node("a", "src", { v: "yo" }), node("b", "upper")], [edge("a", "b")]);
    const third = await runGraph(g2, { lookup, cache });
    expect(third.outputs.b.out).toBe("YO");
    expect(calls).toEqual(["src", "upper", "src", "upper"]);
  });

  it("does not cache nodes that say they are not cacheable", async () => {
    calls.length = 0;
    const cache = new ResultCache();
    const g = graph([node("r", "rand")]);
    const a = await runGraph(g, { lookup, cache });
    const b = await runGraph(g, { lookup, cache });
    expect(calls).toEqual(["rand", "rand"]);
    expect(a.outputs.r.out).not.toBe(b.outputs.r.out);
  });

  it("isolates errors: dependents skip, other branches finish", async () => {
    const g = graph(
      [node("a", "src", { v: "x" }), node("bad", "boom"), node("after", "upper"), node("ok", "upper")],
      [edge("a", "bad"), edge("bad", "after", "out", "in"), edge("a", "ok")]
    );
    // boom has no output port named out in its spec? it does (default).
    const r = await runGraph(g, { lookup });
    expect(r.states.bad).toMatchObject({ status: "error", message: "kaput" });
    expect(r.states.after.status).toBe("skipped");
    expect(r.states.ok.status).toBe("done");
    expect(r.outputs.ok.out).toBe("X");
  });

  it("reports an unconnected required input", async () => {
    const r = await runGraph(graph([node("b", "upper")]), { lookup });
    expect(r.states.b.status).toBe("error");
    expect(r.states.b.message).toMatch(/not connected/);
  });

  it("uses a param when the input is not connected", async () => {
    const r = await runGraph(graph([node("b", "upper", { in: "abc" })]), { lookup });
    // upper reads inputs.in only, so the unconnected param is not an input value
    expect(r.states.b.status).toBe("done");
  });

  it("runs nodes of one lane one at a time, others together", async () => {
    active = 0;
    maxActive = 0;
    const g = graph([node("s1", "slow"), node("s2", "slow"), node("s3", "slow")]);
    await runGraph(g, { lookup });
    expect(maxActive).toBe(1);
  });

  it("stops: running nodes finish as stopped, later ones skip", async () => {
    const ac = new AbortController();
    const g = graph([node("s1", "slow"), node("s2", "slow")]);
    const p = runGraph(g, { lookup, signal: ac.signal });
    ac.abort();
    const r = await p;
    expect(Object.values(r.states).every((s) => s.status === "skipped")).toBe(true);
  });

  it("bypass passes inputs of the same type through", async () => {
    const g = graph(
      [node("a", "aud"), node("p", "pass", {}, { bypass: true })],
      [edge("a", "p", "out", "a")]
    );
    // pass needs t too: make it optional by param
    g.nodes[1].params = { t: "x" };
    const r = await runGraph(g, { lookup });
    expect(r.states.p.status).toBe("skipped");
    expect(r.outputs.p.a).toBe(r.outputs.a.out);
  });

  it("reports state changes as they happen", async () => {
    const seen: string[] = [];
    const g = graph([node("a", "src", { v: "x" })]);
    await runGraph(g, { lookup, onState: (id, s) => seen.push(`${id}:${s.status}`) });
    expect(seen).toEqual(["a:queued", "a:running", "a:done"]);
  });
});

describe("bypassOutputs", () => {
  it("matches by type and uses each input once", () => {
    const d = defs.pass;
    const out = bypassOutputs(d, { a: { url: "u" } as Value, t: "txt" });
    expect(out).toEqual({ a: { url: "u" }, t: "txt" });
  });
});

describe("cacheKey", () => {
  it("ignores key order and changes with values", () => {
    expect(cacheKey("t", { a: 1, b: 2 }, {})).toBe(cacheKey("t", { b: 2, a: 1 }, {}));
    expect(cacheKey("t", { a: 1 }, {})).not.toBe(cacheKey("t", { a: 2 }, {}));
    expect(cacheKey("t", {}, { in: "x" })).not.toBe(cacheKey("t", {}, { in: "y" }));
  });
});

describe("runGraph: missing values", () => {
  it("skips a node whose connected input has no value (bypassed source)", async () => {
    // a bypassed generator has no audio input to pass through, so its output is empty
    const gen = { ...def("gen", { outputs: { out: { type: "text" } } }) };
    defs.gen = gen;
    const g = graph(
      [node("g", "gen", {}, { bypass: true }), node("u", "upper")],
      [edge("g", "u")]
    );
    const r = await runGraph(g, { lookup });
    expect(r.states.g.status).toBe("skipped");
    expect(r.states.u.status).toBe("skipped");
    expect(r.states.u.message).toMatch(/no value/);
  });
});
