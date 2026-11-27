import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem
} from "@/components/ui/select";
import { HandleChange } from "../../types/HandleChange";


export function SelectWithLabel({
  name, label, options, value, onChange,
}: {
  name: string;
  label: string;
  options: { label: string; value: string; }[];
  value: string | undefined;
  onChange: HandleChange;
}) {
  return (
    <div className="flex items-center gap-4">
      <Label>{label}:</Label>
      <Select
        value={value}
        onValueChange={(value) => onChange({ target: { name, value } })}
      >
        <SelectTrigger>
          <SelectValue placeholder="Select a model" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {options.map(({ label, value }) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  );
}
