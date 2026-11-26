import {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  addEdge,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Connection,
  type Edge as FlowEdge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import React from "react";
import { Button } from "../../components/ui/button";
import { ResultCache } from "../cache";
import { runGraph } from "../execute";
import {
  fromFlow,
  hydrate,
  newId,
  parseGraph,
  serializeGraph,
  toFlow,
  type ModuleNode,
} from "../graphIo";
import { autoLayout, suggestPosition } from "../layout";
import { registerBuiltinNodes } from "../nodes";
import { defaultParams, getNodeDef, listNodeDefs } from "../registry";
import { templates } from "../templates";
import type { NodeState, Params, Value } from "../types";
import { canConnect, hasCycle } from "../validate";
import { ChainContext, type ChainContextValue } from "./ChainContext";
import { ModuleNodeView, TYPE_COLOR } from "./ModuleNode";

registerBuiltinNodes();

const STORAGE_KEY = "tts-generation-webui__chains.current.v1";
const nodeTypes = { module: ModuleNodeView };

const readSaved = () => {
  try {
    const text = localStorage.getItem(STORAGE_KEY);
    if (!text) return null;
    const r = parseGraph(text);
    return r.ok ? r.graph : null;
  } catch {
    return null;
  }
};

const CATEGORIES = ["Source", "Generate", "Convert", "Process", "Output"] as const;

const Palette = ({ onAdd }: { onAdd: (type: string) => void }) => {
  const [q, setQ] = React.useState("");
  const defs = listNodeDefs().filter((d) =>
    `${d.label} ${d.category}`.toLowerCase().includes(q.toLowerCase())
  );
  return (
    <aside
      aria-label="Add a node"
      className="flex w-52 shrink-0 flex-col gap-2 overflow-y-auto rounded border bg-background p-2"
    >
      <input
        type="search"
        aria-label="Search nodes"
        placeholder="Search nodes"
        className="w-full rounded border border-input bg-background px-2 py-1 text-sm"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      {CATEGORIES.map((cat) => {
        const items = defs.filter((d) => d.category === cat);
        if (!items.length) return null;
        return (
          <div key={cat} className="flex flex-col gap-1">
            <h3 className="text-xs font-semibold uppercase text-muted-foreground">{cat}</h3>
            {items.map((d) => (
              <button
                key={d.type}
                type="button"
                className="rounded border px-2 py-1 text-left text-sm hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => onAdd(d.type)}
              >
                {d.label}
              </button>
            ))}
          </div>
        );
      })}
    </aside>
  );
};

const Editor = () => {
  const initial = React.useMemo(
    () => hydrate(readSaved() ?? templates[0].graph),
    []
  );
  const flow0 = React.useMemo(() => toFlow(initial), [initial]);
  const [nodes, setNodes, onNodesChange] = useNodesState<ModuleNode>(flow0.nodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState<FlowEdge>(flow0.edges);
  const [states, setStates] = React.useState<Record<string, NodeState>>({});
  const [outputs, setOutputs] = React.useState<Record<string, Record<string, Value>>>({});
  const [running, setRunning] = React.useState(false);
  const [problems, setProblems] = React.useState<string[]>([]);
  const cache = React.useRef(new ResultCache());
  const abort = React.useRef<AbortController | null>(null);
  const fileInput = React.useRef<HTMLInputElement>(null);
  const rf = useReactFlow();

  // Save as you go. Storage can be blocked, so a failure is not an error.
  React.useEffect(() => {
    const t = setTimeout(() => {
      try {
        localStorage.setItem(STORAGE_KEY, serializeGraph(fromFlow(nodes, edges)));
      } catch {
        /* storage is off */
      }
    }, 400);
    return () => clearTimeout(t);
  }, [nodes, edges]);

  const connected = React.useMemo(
    () => new Set(edges.map((e) => `${e.target}:${e.targetHandle}`)),
    [edges]
  );

  const valueAt = React.useCallback(
    (nodeId: string, port: string): Value => {
      const e = edges.find((x) => x.target === nodeId && x.targetHandle === port);
      return e ? outputs[e.source]?.[e.sourceHandle ?? ""] : undefined;
    },
    [edges, outputs]
  );

  const setParam = React.useCallback(
    (id: string, key: string, value: Params[string]) =>
      setNodes((ns) =>
        ns.map((n) =>
          n.id === id
            ? { ...n, data: { ...n.data, params: { ...n.data.params, [key]: value } } }
            : n
        )
      ),
    [setNodes]
  );
  const toggleBypass = React.useCallback(
    (id: string) =>
      setNodes((ns) =>
        ns.map((n) =>
          n.id === id ? { ...n, data: { ...n.data, bypass: !n.data.bypass } } : n
        )
      ),
    [setNodes]
  );
  const removeNode = React.useCallback(
    (id: string) => {
      setNodes((ns) => ns.filter((n) => n.id !== id));
      setEdges((es) => es.filter((e) => e.source !== id && e.target !== id));
      cache.current.delete(id);
    },
    [setNodes, setEdges]
  );

  const portType = React.useCallback(
    (nodeId: string, port: string | null | undefined, side: "in" | "out") => {
      const n = nodes.find((x) => x.id === nodeId);
      const def = n && getNodeDef(n.data.type);
      return port && def
        ? (side === "in" ? def.inputs : def.outputs)[port]?.type
        : undefined;
    },
    [nodes]
  );

  const isValidConnection = React.useCallback(
    (c: Connection | FlowEdge) => {
      if (c.source === c.target) return false;
      const a = portType(c.source, c.sourceHandle, "out");
      const b = portType(c.target, c.targetHandle, "in");
      if (!a || !b || !canConnect(a, b)) return false;
      const candidate = fromFlow(nodes, [
        ...edges.filter(
          (e) => !(e.target === c.target && e.targetHandle === c.targetHandle)
        ),
        { id: "candidate", source: c.source, target: c.target, sourceHandle: c.sourceHandle, targetHandle: c.targetHandle },
      ]);
      return !hasCycle(candidate);
    },
    [nodes, edges, portType]
  );

  const onConnect = React.useCallback(
    (c: Connection) =>
      setEdges((es) =>
        addEdge(
          { ...c, id: newId("e") },
          // an input takes one cable: a new one replaces the old
          es.filter((e) => !(e.target === c.target && e.targetHandle === c.targetHandle))
        )
      ),
    [setEdges]
  );

  const styledEdges = React.useMemo(
    () =>
      edges.map((e) => {
        const t = portType(e.source, e.sourceHandle, "out");
        return {
          ...e,
          animated: states[e.source]?.status === "running",
          style: { stroke: t ? TYPE_COLOR[t] : "#9ca3af", strokeWidth: 2.5 },
        };
      }),
    [edges, portType, states]
  );

  const addNode = (type: string) => {
    const def = getNodeDef(type);
    if (!def) return;
    const id = newId();
    // computed from the latest list, so quick repeated clicks do not overlap
    setNodes((ns) => [
      ...ns,
      {
        id,
        type: "module",
        position: suggestPosition(
          def,
          ns.map((n) => ({ ...n.position, height: n.measured?.height }))
        ),
        data: { type, params: defaultParams(def) },
      },
    ]);
    // the node is measured on its first paint; then bring it into view
    setTimeout(() => rf.fitView({ padding: "12%", duration: 200 }), 120);
  };

  const loadGraph = (g: ReturnType<typeof hydrate>) => {
    const f = toFlow(g);
    setNodes(f.nodes);
    setEdges(f.edges);
    setStates({});
    setOutputs({});
    setProblems([]);
    cache.current.clear();
    setTimeout(() => rf.fitView({ padding: "12%" }), 50);
  };

  const run = async () => {
    const graph = fromFlow(nodes, edges);
    const ac = new AbortController();
    abort.current = ac;
    setRunning(true);
    setProblems([]);
    setStates({});
    try {
      const r = await runGraph(graph, {
        signal: ac.signal,
        cache: cache.current,
        onState: (id, s) => setStates((x) => ({ ...x, [id]: s })),
        onOutputs: (id, o) => setOutputs((x) => ({ ...x, [id]: o })),
      });
      if (r.problems) setProblems(r.problems);
    } finally {
      setRunning(false);
      abort.current = null;
    }
  };

  const exportFile = () => {
    const blob = new Blob([serializeGraph(fromFlow(nodes, edges))], {
      type: "application/json",
    });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "chain.json";
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  };

  const importFile = async (file?: File) => {
    if (!file) return;
    const r = parseGraph(await file.text());
    if (!r.ok) {
      setProblems(["Could not open that file:", ...r.problems]);
      return;
    }
    loadGraph(r.graph);
  };

  const confirmReplace = () =>
    nodes.length === 0 || window.confirm("Replace the current chain?");

  const layout = () => {
    const heights = Object.fromEntries(
      nodes.flatMap((n) => (n.measured?.height ? [[n.id, n.measured.height]] : []))
    );
    const pos = autoLayout(fromFlow(nodes, edges), getNodeDef, heights);
    setNodes((ns) => ns.map((n) => (pos[n.id] ? { ...n, position: pos[n.id] } : n)));
    setTimeout(() => rf.fitView({ padding: "12%" }), 50);
  };

  const ctx: ChainContextValue = {
    states,
    outputs,
    connected,
    valueAt,
    setParam,
    toggleBypass,
    removeNode,
    running,
  };

  return (
    <ChainContext.Provider value={ctx}>
      <div className="flex w-full flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2" role="toolbar" aria-label="Chain actions">
          <Button onClick={run} disabled={running || nodes.length === 0}>
            {running ? "Running…" : "Run chain"}
          </Button>
          <Button variant="outline" onClick={() => abort.current?.abort()} disabled={!running}>
            Stop
          </Button>
          <select
            aria-label="Start from a template"
            className="h-10 rounded-md border border-input bg-background px-2 text-sm"
            value=""
            onChange={(e) => {
              const t = templates[Number(e.target.value)];
              if (t && confirmReplace()) loadGraph(hydrate(t.graph));
            }}
          >
            <option value="" disabled>
              Templates…
            </option>
            {templates.map((t, i) => (
              <option key={t.name} value={i} title={t.description}>
                {t.name}
              </option>
            ))}
          </select>
          <Button variant="outline" onClick={layout} disabled={running}>
            Tidy up
          </Button>
          <Button variant="outline" onClick={exportFile}>
            Export
          </Button>
          <Button variant="outline" onClick={() => fileInput.current?.click()} disabled={running}>
            Import
          </Button>
          <input
            ref={fileInput}
            type="file"
            accept="application/json,.json"
            hidden
            onChange={(e) => {
              importFile(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
          <Button
            variant="outline"
            disabled={running || nodes.length === 0}
            onClick={() => {
              if (window.confirm("Remove everything from the chain?")) loadGraph({ version: 1, nodes: [], edges: [] });
            }}
          >
            Clear
          </Button>
        </div>

        {problems.length > 0 && (
          <div role="alert" className="rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800">
            <ul className="list-disc pl-5">
              {problems.map((p, i) => (
                <li key={i}>{p}</li>
              ))}
            </ul>
          </div>
        )}

        <div className="flex h-[72vh] min-h-[420px] w-full gap-2">
          <Palette onAdd={addNode} />
          <div className="relative min-w-0 flex-1 rounded border">
            <ReactFlow
              nodes={nodes}
              edges={styledEdges}
              nodeTypes={nodeTypes}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              isValidConnection={isValidConnection}
              fitView
              fitViewOptions={{ padding: "12%" }}
              minZoom={0.2}
              maxZoom={1.5}
              snapToGrid
              snapGrid={[20, 20]}
              deleteKeyCode={["Backspace", "Delete"]}
              proOptions={{ hideAttribution: false }}
            >
              <Background variant={BackgroundVariant.Dots} gap={20} />
              <Controls showInteractive={false} />
              <MiniMap pannable zoomable ariaLabel="Chain overview" />
            </ReactFlow>
            {nodes.length === 0 && (
              <p className="pointer-events-none absolute inset-0 flex items-center justify-center text-muted-foreground">
                Add a node from the list, or pick a template.
              </p>
            )}
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          Cables run left to right. Drag from a dot to another node's dot of the same colour. Audio
          from the browser nodes (Gain, Trim, Fade, Join, Mix) is uploaded before a server node uses it.
        </p>
      </div>
    </ChainContext.Provider>
  );
};

export const ChainsEditor = () => (
  <ReactFlowProvider>
    <Editor />
  </ReactFlowProvider>
);

export default ChainsEditor;
