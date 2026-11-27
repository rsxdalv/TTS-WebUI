import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectItem,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { HandleChange } from "../../types/HandleChange";
import { ServerOffIcon } from "lucide-react";

export function ModelDropdown({
  name = "model_name",
  label = "Model",
  options,
  value,
  onChange,
  onRefresh,
  onOpen,
  onUnload,
  loading,
}: {
  name: string;
  label?: string;
  options: string[];
  value: string;
  onChange: HandleChange;
  onRefresh?: () => void;
  onOpen?: () => void;
  onUnload?: () => void;
  loading?: boolean;
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
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={onOpen}
          title="Open Folder"
        >
          <FolderIcon className="h-5 w-5" />
          <span className="sr-only">Open Folder</span>
        </Button>
        <Button variant="ghost" size="icon" onClick={onRefresh} title="Refresh">
          <RefreshCwIcon
            className={loading ? "h-5 w-5 animate-spin" : "h-5 w-5"}
          />
          <span className="sr-only">Refresh List</span>
        </Button>
        {onUnload && <UnloadModelButton onUnload={onUnload} />}
      </div>
    </div>
  );
}

export const UnloadModelButton = ({ onUnload }: { onUnload: () => void }) => (
  <Button variant="ghost" size="icon" onClick={onUnload} title="Unload Model">
    <ServerOffIcon className="h-5 w-5" />
    <span className="sr-only">Unload Model</span>
  </Button>
);

function TrashIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M3 6h18" />
      <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
      <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
    </svg>
  );
}
function FolderIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
    </svg>
  );
}

function RefreshCwIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
      <path d="M21 3v5h-5" />
      <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
      <path d="M8 16H3v5" />
    </svg>
  );
}
