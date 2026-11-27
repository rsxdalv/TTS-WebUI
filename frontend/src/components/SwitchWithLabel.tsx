import { JSX } from "react";
import { HandleChange } from "../types/HandleChange";
import { Label } from "./ui/label";
import { Switch } from "./ui/switch";

export const SwitchWithLabel = ({
  label,
  name,
  value,
  onChange,
  disabled,
}: {
  label: string | JSX.Element;
  name: string;
  value: boolean | undefined;
  onChange: HandleChange;
  disabled?: boolean;
}) => (
  <div className="flex gap-2 items-center">
    <Switch
      id={name}
      checked={value}
      disabled={disabled}
      onCheckedChange={(value) =>
        onChange({
          target: {
            name,
            value,
          },
        })
      }
    />
    <Label htmlFor={name}>{label}</Label>
  </div>
);
