import React from "react";
import { HandleChange } from "../types/HandleChange";
import { Label } from "./ui/label";
import { cn } from "../lib/utils";
import { Textarea } from "./ui/textarea";

export const PromptTextArea = <
  T extends { [key: string]: number | string | boolean },
>({
  params,
  handleChange,
  label,
  name,
  className,
  optimalLength,
  softMaxLength,
  hardMaxLength,
  placeholder,
  rows,
}: {
  params: T;
  handleChange: HandleChange;
  label: string;
  name: keyof T;
  className?: string;
  optimalLength?: number;
  softMaxLength?: number;
  hardMaxLength?: number;
  placeholder?: string;
  rows?: number;
}) => (
  <div className={cn("flex flex-col gap-2", className)}>
    <Label htmlFor={name as string}>
      {label && `${label}:`}{label && " "}
      {optimalLength && (
        <span className="text-xs text-slate-500">
          (Optimal length per generation/line: {optimalLength} characters)
        </span>
      )}
    </Label>
    <Textarea
      id={name as string}
      name={name as string}
      value={params[name] as string}
      onChange={handleChange}
      className="cell"
      placeholder={placeholder || "Enter text here..."}
      rows={rows || 3}
      // maxLength={hardMaxLength || 100000}
    />
  </div>
);
