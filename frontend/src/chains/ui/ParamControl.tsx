import React from "react";
import { Slider } from "../../components/ui/slider";
import { Switch } from "../../components/ui/switch";
import type { ParamSpec, Params } from "../types";

const field =
  "nodrag nowheel w-full rounded border border-input bg-background px-2 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50";

/** The number box beside a slider: shows the value and takes typed values. */
const NumberBox = ({
  value,
  min,
  max,
  step,
  onChange,
  label,
  disabled,
}: {
  value: number;
  min?: number;
  max?: number;
  step?: number;
  onChange: (v: number) => void;
  label: string;
  disabled?: boolean;
}) => (
  <input
    type="number"
    aria-label={label}
    className="nodrag nowheel w-24 shrink-0 rounded border border-input bg-background px-2 py-1 text-right text-sm tabular-nums focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
    value={Number.isFinite(value) ? value : 0}
    min={min}
    max={max}
    step={step}
    disabled={disabled}
    onChange={(e) => {
      const v = parseFloat(e.target.value);
      if (!Number.isNaN(v)) onChange(v);
    }}
  />
);

export const ParamControl = ({
  spec,
  value,
  onChange,
  fed,
}: {
  spec: ParamSpec;
  value: Params[string];
  onChange: (v: Params[string]) => void;
  /** A cable feeds this param, so the typed value is not used. */
  fed?: boolean;
}) => {
  const id = React.useId();
  const label = (
    <label htmlFor={id} className="text-xs text-muted-foreground">
      {spec.label}
      {fed && <span className="ml-1 italic">(from cable)</span>}
    </label>
  );
  switch (spec.kind) {
    case "textarea":
      return (
        <div className="flex flex-col gap-1">
          {label}
          <textarea
            id={id}
            rows={3}
            className={`${field} resize-y`}
            value={String(value ?? "")}
            disabled={fed}
            onChange={(e) => onChange(e.target.value)}
          />
        </div>
      );
    case "text":
      return (
        <div className="flex flex-col gap-1">
          {label}
          <input
            id={id}
            type="text"
            className={field}
            value={String(value ?? "")}
            disabled={fed}
            onChange={(e) => onChange(e.target.value)}
          />
        </div>
      );
    case "select":
      return (
        <div className="flex flex-col gap-1">
          {label}
          <select
            id={id}
            className={field}
            value={String(value)}
            onChange={(e) => onChange(e.target.value)}
          >
            {spec.options.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </select>
        </div>
      );
    case "toggle":
      return (
        <div className="flex items-center justify-between gap-2 nodrag">
          {label}
          <Switch
            id={id}
            checked={Boolean(value)}
            onCheckedChange={(c) => onChange(c)}
          />
        </div>
      );
    case "number":
      return (
        <div className="flex items-center justify-between gap-2">
          {label}
          <NumberBox
            label={spec.label}
            value={Number(value)}
            min={spec.min}
            max={spec.max}
            step={spec.step}
            disabled={fed}
            onChange={onChange}
          />
        </div>
      );
    case "slider":
      return (
        <div className="flex flex-col gap-0">
          <div className="flex items-center justify-between gap-2">
            {label}
            <NumberBox
              label={spec.label}
              value={Number(value)}
              min={spec.min}
              max={spec.max}
              step={spec.step}
              onChange={onChange}
            />
          </div>
          <Slider
            className="nodrag nowheel !my-2"
            aria-label={spec.label}
            min={spec.min}
            max={spec.max}
            step={spec.step}
            value={[Number(value)]}
            onValueChange={([v]) => onChange(v)}
          />
        </div>
      );
  }
};
