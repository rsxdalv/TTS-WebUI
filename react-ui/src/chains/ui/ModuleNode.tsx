import { Handle, Position, type NodeProps } from "@xyflow/react";
import React from "react";
import type { ModuleData, ModuleNode as ModuleNodeType } from "../graphIo";
import { getNodeDef } from "../registry";
import type { AudioValue, NodeStatus, PortSpec, Value, ValueType } from "../types";
import { useChain } from "./ChainContext";
import { ParamControl } from "./ParamControl";

export const TYPE_COLOR: Record<ValueType, string> = {
  audio: "#2563eb",
  text: "#16a34a",
  number: "#d97706",
};

const STATUS_LABEL: Record<NodeStatus, string> = {
  idle: "Ready",
  queued: "Waiting",
  running: "Running",
  done: "Done",
  cached: "Unchanged",
  error: "Error",
  skipped: "Skipped",
};

const STATUS_COLOR: Record<NodeStatus, string> = {
  idle: "#9ca3af",
  queued: "#9ca3af",
  running: "#2563eb",
  done: "#16a34a",
  cached: "#16a34a",
  error: "#dc2626",
  skipped: "#d97706",
};

const isAudio = (v: Value): v is AudioValue =>
  !!v && typeof v === "object" && "url" in v;

const AudioPlayer = ({ value, label }: { value: Value; label?: string }) =>
  isAudio(value) ? (
    <div className="flex flex-col gap-0.5">
      {label && <span className="text-xs text-muted-foreground">{label}</span>}
      {/* The native player: light, keyboard accessible, and it needs no setup. */}
      <audio
        className="nodrag nowheel h-9 w-full"
        controls
        preload="metadata"
        src={value.url}
        aria-label={label ?? value.name ?? "Audio"}
      />
    </div>
  ) : null;

const PortRow = ({
  nodeId,
  input,
  output,
}: {
  nodeId: string;
  input?: [string, PortSpec];
  output?: [string, PortSpec];
}) => (
  <div className="relative flex items-center justify-between text-xs min-h-[22px]">
    <span className="pl-1">
      {input && (
        <>
          <Handle
            id={input[0]}
            type="target"
            position={Position.Left}
            aria-label={`Input ${input[1].label ?? input[0]} (${input[1].type})`}
            style={{ background: TYPE_COLOR[input[1].type], width: 12, height: 12, left: -7 }}
          />
          {input[1].label ?? input[0]}
        </>
      )}
    </span>
    <span className="pr-1 text-right">
      {output && (
        <>
          {output[1].label ?? output[0]}
          <Handle
            id={output[0]}
            type="source"
            position={Position.Right}
            aria-label={`Output ${output[1].label ?? output[0]} (${output[1].type})`}
            style={{ background: TYPE_COLOR[output[1].type], width: 12, height: 12, right: -7 }}
          />
        </>
      )}
    </span>
  </div>
);

export const ModuleNodeView = ({ id, data, selected }: NodeProps<ModuleNodeType>) => {
  const chain = useChain();
  const d = data as ModuleData;
  const def = getNodeDef(d.type);
  if (!def) {
    return (
      <div className="w-[300px] rounded-md border-2 border-dashed border-red-500 bg-background p-3 text-sm">
        Missing node type <code>{d.type}</code>. Its extension is not installed.
      </div>
    );
  }
  const state = chain.states[id] ?? { status: "idle" as NodeStatus };
  const ins = Object.entries(def.inputs);
  const outs = Object.entries(def.outputs);
  const rows = Math.max(ins.length, outs.length);
  const outputs = chain.outputs[id] ?? {};
  const showInputs = def.category === "Output";

  return (
    <div
      className={`w-[300px] rounded-md border bg-background shadow-sm ${
        selected ? "ring-2 ring-ring" : ""
      } ${d.bypass ? "opacity-60" : ""}`}
      aria-label={def.label}
    >
      <div className="flex items-center justify-between gap-2 rounded-t-md border-b bg-secondary px-2 py-1.5">
        <span className="truncate text-sm font-semibold">{def.label}</span>
        <span className="flex items-center gap-2">
          <span
            className="flex items-center gap-1 text-xs"
            role="status"
            aria-live="polite"
            title={state.message}
          >
            <span
              aria-hidden
              className={`inline-block h-2.5 w-2.5 rounded-full ${
                state.status === "running" ? "animate-pulse" : ""
              }`}
              style={{ background: STATUS_COLOR[state.status] }}
            />
            {STATUS_LABEL[state.status]}
          </span>
          <button
            type="button"
            className={`nodrag rounded border px-1.5 text-xs hover:bg-accent ${
              d.bypass ? "bg-primary text-primary-foreground hover:bg-primary/90" : ""
            }`}
            aria-pressed={!!d.bypass}
            title="Pass audio through without processing"
            onClick={() => chain.toggleBypass(id)}
          >
            Bypass
          </button>
          <button
            type="button"
            className="nodrag rounded border px-1.5 text-xs hover:bg-accent"
            aria-label={`Remove ${def.label}`}
            disabled={chain.running}
            onClick={() => chain.removeNode(id)}
          >
            ✕
          </button>
        </span>
      </div>

      {state.status === "error" && (
        <div className="border-b bg-red-50 px-2 py-1 text-xs text-red-700" role="alert">
          {state.message}
        </div>
      )}
      {state.status === "running" && state.progress !== undefined && state.message && (
        <div className="border-b px-2 py-1 text-xs text-muted-foreground">{state.message}</div>
      )}

      {rows > 0 && (
        <div className="flex flex-col gap-0.5 px-1 py-1.5">
          {Array.from({ length: rows }, (_, i) => (
            <PortRow key={i} nodeId={id} input={ins[i]} output={outs[i]} />
          ))}
        </div>
      )}

      {def.params.length > 0 && (
        <div className="flex flex-col gap-2 border-t px-2 py-2">
          {def.params.map((spec) => (
            <ParamControl
              key={spec.key}
              spec={spec}
              value={d.params[spec.key]}
              fed={chain.connected.has(`${id}:${spec.key}`)}
              onChange={(v) => chain.setParam(id, spec.key, v)}
            />
          ))}
        </div>
      )}

      {(showInputs ? ins.length > 0 : outs.some(([, s]) => s.type === "audio")) && (
        <div className="flex flex-col gap-1.5 border-t px-2 py-2">
          {showInputs
            ? ins.map(([port, spec]) => {
                const v = chain.valueAt(id, port);
                if (spec.type === "audio") {
                  return (
                    <AudioPlayer key={port} value={v} label={spec.label ?? port} />
                  );
                }
                return v == null ? null : (
                  <pre
                    key={port}
                    className="nodrag nowheel max-h-32 overflow-auto whitespace-pre-wrap rounded bg-secondary p-1.5 text-xs"
                  >
                    {String(v)}
                  </pre>
                );
              })
            : outs
                .filter(([, s]) => s.type === "audio")
                .map(([port, spec]) => (
                  <AudioPlayer
                    key={port}
                    value={outputs[port]}
                    label={outs.length > 1 ? spec.label ?? port : undefined}
                  />
                ))}
          {showInputs && ins.every(([p]) => chain.valueAt(id, p) == null) && (
            <span className="text-xs text-muted-foreground">Run the chain to listen.</span>
          )}
        </div>
      )}
    </div>
  );
};
