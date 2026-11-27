import { beforeAll, describe, expect, it } from "vitest";
import { fromFlow, parseGraph, serializeGraph, toFlow } from "../graphIo";
import { autoLayout, estimateHeight } from "../layout";
import { registerBuiltinNodes } from "../nodes";
import { getNodeDef, listNodeDefs, defaultParams } from "../registry";
import { templates } from "../templates";
import { validateGraph } from "../validate";

beforeAll(() => registerBuiltinNodes());

describe("built-in nodes", () => {
  it("have unique types, matching param defaults and valid ports", () => {
    const types = listNodeDefs().map((d) => d.type);
    expect(new Set(types).size).toBe(types.length);
    expect(types.length).toBeGreaterThanOrEqual(14);
    for (const d of listNodeDefs()) {
      const keys = d.params.map((p) => p.key);
      expect(new Set(keys).size).toBe(keys.length);
      for (const p of d.params) {
        if (p.kind === "select") expect(p.options).toContain(p.default);
        if (p.kind === "slider") {
          expect(p.default).toBeGreaterThanOrEqual(p.min);
          expect(p.default).toBeLessThanOrEqual(p.max);
        }
      }
      // an input that also has a param key must be a text or number port
      for (const [name, spec] of Object.entries(d.inputs)) {
        if (keys.includes(name)) expect(spec.type).not.toBe("audio");
      }
    }
  });
});

describe("templates", () => {
  for (const t of templates) {
    it(`"${t.name}" is a valid graph`, () => {
      expect(validateGraph(t.graph, getNodeDef)).toEqual([]);
      // and survives the round trip through text
      const back = parseGraph(serializeGraph(t.graph));
      expect(back.ok).toBe(true);
    });
  }
});

describe("parseGraph", () => {
  const good = templates[0].graph;
  it("round-trips and fills defaults", () => {
    const r = parseGraph(serializeGraph(good));
    if (!r.ok) throw new Error(r.problems.join());
    const mms = r.graph.nodes.find((n) => n.type === "tts.mms")!;
    expect(mms.params).toEqual(defaultParams(getNodeDef("tts.mms")!));
  });
  it("drops unknown params and wrong types", () => {
    const g = structuredClone(good);
    g.nodes[1].params = { language: 5, speaking_rate: "fast", evil: "x", noise_scale: 1.2 } as any;
    const r = parseGraph(JSON.stringify(g));
    if (!r.ok) throw new Error(r.problems.join());
    const p = r.graph.nodes[1].params;
    expect(p.evil).toBeUndefined();
    expect(p.language).toBe("eng");
    expect(p.speaking_rate).toBe(1);
    expect(p.noise_scale).toBe(1.2);
  });
  it("refuses select values outside the options", () => {
    const g = structuredClone(templates[2].graph);
    g.nodes[2].params = { device: "rm -rf /" } as any;
    const r = parseGraph(JSON.stringify(g));
    if (!r.ok) throw new Error();
    expect(r.graph.nodes[2].params.device).toBe("cuda:0");
  });
  it("refuses bad JSON, big files, and code-looking node types", () => {
    expect(parseGraph("{").ok).toBe(false);
    expect(parseGraph(" ".repeat(2 * 1024 * 1024 + 1)).ok).toBe(false);
    const g: any = structuredClone(good);
    g.nodes[0].type = "custom.js";
    g.nodes[0].params = { code: "alert(1)" };
    const r = parseGraph(JSON.stringify(g));
    expect(r.ok).toBe(false);
  });
  it("is not fooled by __proto__ keys in params", () => {
    const g: any = structuredClone(good);
    const text = JSON.stringify(g).replace('"params":{}', '"params":{"__proto__":{"polluted":true}}');
    parseGraph(text);
    expect(({} as any).polluted).toBeUndefined();
  });
});

describe("flow conversion", () => {
  it("round-trips through React Flow shapes", () => {
    const g = templates[1].graph;
    const { nodes, edges } = toFlow(g);
    const back = fromFlow(nodes, edges);
    expect(back.nodes.map((n) => n.id)).toEqual(g.nodes.map((n) => n.id));
    expect(back.edges).toEqual(g.edges);
  });
  it("drops edges without handles and keeps bypass", () => {
    const { nodes, edges } = toFlow(templates[0].graph);
    nodes[1].data.bypass = true;
    const back = fromFlow(nodes, [...edges, { id: "x", source: "t", target: "s" }]);
    expect(back.edges.length).toBe(3);
    expect(back.nodes[1].bypass).toBe(true);
  });
});

describe("autoLayout", () => {
  it("puts each node right of what feeds it and keeps columns apart", () => {
    const pos = autoLayout(templates[1].graph, getNodeDef);
    expect(pos.t.x).toBe(0);
    expect(pos.k.x).toBeGreaterThan(pos.t.x);
    expect(pos.c.x).toBeGreaterThan(pos.k.x);
    expect(pos.k.x).toBe(pos.m.x);
    expect(Math.abs(pos.k.y - pos.m.y)).toBeGreaterThanOrEqual(estimateHeight(getNodeDef("tts.kokoro")));
  });
  it("returns nothing for a cyclic graph", () => {
    const g = structuredClone(templates[0].graph);
    g.edges.push({ id: "back", from: ["p", "audio"], to: ["t", "text"] });
    expect(autoLayout(g, getNodeDef)).toEqual({});
  });
});

describe("autoLayout: unconnected nodes and measured heights", () => {
  const lone = (id: string, type: string) => ({ id, type, x: 0, y: 0, params: {} });
  it("places nodes with no cables by category, left to right", () => {
    const g = { version: 1 as const, nodes: [lone("p", "out.preview"), lone("t", "source.text"), lone("m", "tts.mms"), lone("x", "fx.gain")], edges: [] };
    const pos = autoLayout(g, getNodeDef);
    expect(pos.t.x).toBeLessThan(pos.m.x);
    expect(pos.m.x).toBeLessThan(pos.x.x);
    expect(pos.x.x).toBeLessThan(pos.p.x);
  });
  it("uses measured heights so tall nodes do not overlap", () => {
    const g = { version: 1 as const, nodes: [lone("a", "source.text"), lone("b", "source.text")], edges: [] };
    const pos = autoLayout(g, getNodeDef, { a: 700, b: 100 });
    const [first, second] = pos.a.y <= pos.b.y ? ["a", "b"] : ["b", "a"];
    expect(pos[second].y - pos[first].y).toBeGreaterThanOrEqual(700 * (first === "a" ? 1 : 0) + 100 * (first === "b" ? 1 : 0));
  });
});

import { suggestPosition, NODE_WIDTH, COLUMN_GAP } from "../layout";
describe("suggestPosition", () => {
  const col = NODE_WIDTH + COLUMN_GAP;
  it("starts at the top of its kind's column", () => {
    expect(suggestPosition(getNodeDef("tts.mms")!, [])).toEqual({ x: col, y: 0 });
    expect(suggestPosition(getNodeDef("out.preview")!, [])).toEqual({ x: 4 * col, y: 0 });
  });
  it("goes below nodes already in that column, never on top", () => {
    const p = suggestPosition(getNodeDef("tts.mms")!, [
      { x: col, y: 0, height: 400 },
      { x: col, y: 440, height: 200 },
      { x: 0, y: 900, height: 100 }, // another column: ignored
    ]);
    expect(p.x).toBe(col);
    expect(p.y).toBeGreaterThanOrEqual(640);
  });
});
