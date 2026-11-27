import React, { JSX } from "react";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Slider } from "@/components/ui/slider";
import { HandleChange } from "../types/HandleChange";
import { cn } from "../lib/utils";

function countDigits(min, max) {
  const countDigitsInNumber = (num) => num.toFixed(0).length;
  return Math.max(countDigitsInNumber(min), countDigitsInNumber(max));
}

const LabelledSlider = ({
  min,
  max,
  name,
  label,
  value,
  decimals = 0,
  className,
  onChange,
  orientation = "horizontal",
  ...props
}: {
  min: number;
  max: number;
  step: number;
  className?: string;
  label: string | JSX.Element;
  name: string;
  decimals?: number;
  value: number;
  onChange: HandleChange;
  disabled?: boolean;
  orientation?: "horizontal" | "vertical";
}) => (
  <div
    className={
      orientation === "vertical"
        ? "flex flex-col gap-2 items-center"
        : "grid gap-2 grid-cols-2 items-center w-full"
    }
  >
    <Label
      htmlFor={name}
      className={cn(
        "justify-self-start order-first",
        orientation === "vertical" && "text-center"
      )}
    >
      {label}
    </Label>

    <Input
      type="number"
      id={name}
      name={name}
      min={min}
      max={max}
      {...props}
      value={value}
      onChange={onChange}
      className={cn(
        "text-right justify-self-end",
        orientation === "vertical" && "order-last"
      )}
      style={{ width: `${(countDigits(min, max) + decimals) * 1.5 + 4}ch` }}
    />

    <div
      className={cn(orientation === "horizontal" && "col-span-2", className)}
    >
      <Slider
        min={min}
        max={max}
        name={name}
        {...props}
        value={[value]}
        orientation={orientation}
        onValueChange={(values) =>
          onChange({
            target: {
              name,
              value: values[0].toString(),
              type: "number",
            },
          })
        }
      >
        <div>
          <div className="bg-primary" />
        </div>
        <div className="bg-primary-foreground" />
      </Slider>
    </div>
    <div
      className={cn(
        "flex justify-between text-sm text-muted-foreground",
        orientation === "horizontal" && "col-span-2",
        orientation === "vertical" && "hidden"
      )}
    >
      <span>{min.toFixed(decimals)}</span>
      <span>{max.toFixed(decimals)}</span>
    </div>
    <span
      className={cn(
        "flex justify-between text-sm text-muted-foreground",
        orientation === "vertical" && "col-span-2",
        orientation === "horizontal" && "hidden"
      )}
    >
      {min.toFixed(decimals)}
    </span>
    <span
      className={cn(
        "flex justify-between text-sm text-muted-foreground",
        orientation === "vertical" && "col-span-2 -order-1",
        orientation === "horizontal" && "hidden"
      )}
    >
      {max.toFixed(decimals)}
    </span>
  </div>
);

export const ParameterSlider = <
  T extends {
    [key: string]: number | string | boolean;
  },
>({
  params,
  name,
  min,
  max,
  step,
  decimals = 0,
  disabled = false,
  // onChange,
  // className,
  // label,
  ...props
}: {
  params: T;
  onChange: HandleChange;
  label: string | JSX.Element;
  name: keyof T;
  min: string | number;
  max: string | number;
  step: string | number;
  className?: string;
  decimals?: number;
  disabled?: boolean;
  orientation?: "horizontal" | "vertical";
}) => (
  <LabelledSlider
    value={params[name] as number}
    min={Number(min)}
    max={Number(max)}
    step={Number(step)}
    name={name as string}
    decimals={decimals}
    disabled={disabled}
    // onChange={onChange}
    // className={className}
    // label={label}
    {...props}
  />
);
