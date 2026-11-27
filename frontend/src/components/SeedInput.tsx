import React from "react";
import { HandleChange } from "../types/HandleChange";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { cn } from "../lib/utils";
import { Input } from "./ui/input";

export const SeedInput = <
  T extends {
    use_random_seed?: boolean;
    seed: number | string | undefined;
  },
>({
  params,
  handleChange,
  className,
}: {
  params: T;
  handleChange: HandleChange;
  className?: string;
}) => (
  // <div className={cn("flex gap-2 items-center w-full flex-wrap", className)}>
  <div className={cn("flex gap-2 items-center w-full", className)}>
    <Label htmlFor="seed">Seed:</Label>
    <Input
      type="number"
      name="seed"
      min={0}
      // max={2 ** 32 - 1}
      disabled={params.use_random_seed}
      value={params.seed}
      onChange={handleChange}
      maxLength={10}
      className="text-right w-36"
    />

    <Switch
      id="randomize"
      checked={params.use_random_seed}
      onCheckedChange={(value) =>
        handleChange({
          target: {
            name: "use_random_seed",
            value,
          },
        })
      }
    />
    <Label htmlFor="randomize">Randomize</Label>
  </div>
);
