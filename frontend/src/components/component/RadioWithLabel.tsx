import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { cn } from "../../lib/utils";

export function RadioWithLabel({
  options,
  name,
  onChange,
  label = "",
  inline = false,
  variant = "vertical",
  className = "",
  ...props
  // }: {
  //   options: { label: string; value: string; disabled?: boolean }[];
  //   name: string;
  //   onChange: HandleChange;
  //   label?: string;
  //   inline?: boolean;
  //   horizontal?: boolean;
  // } & Omit<React.HTMLAttributes<HTMLDivElement>, "onChange">) {
}) {
  const variantWithInline = inline ? "inline" : variant;
  return (
    <div
      className={cn(
        "flex",
        {
          inline: "flex-row items-center gap-4",
          vertical: "flex-col gap-2",
          horizontal: "flex-col gap-2",
        }[variantWithInline],
        className
      )}
    >
      {label && (
        <Label htmlFor={name} className="flex-shrink-0">
          {label}:
        </Label>
      )}
      <RadioGroup
        id={name}
        name={name}
        onValueChange={(value) => onChange({ target: { name, value } })}
        {...props}
        className={cn(
          "flex",
          {
            inline: "flex-row items-center gap-4 w-full flex-wrap",
            vertical: "flex-col",
            horizontal: "flex-row items-center gap-4 w-full flex-wrap",
          }[variantWithInline]
        )}
      >
        {options.map((option) => (
          <div className="flex items-center gap-2" key={option.value}>
            <RadioGroupItem
              id={option.value + name}
              value={option.value}
              disabled={option.disabled}
              className="peer"
            />
            <Label
              htmlFor={option.value + name}
              className="peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              {option.label}
            </Label>
          </div>
        ))}
      </RadioGroup>
    </div>
  );
}
