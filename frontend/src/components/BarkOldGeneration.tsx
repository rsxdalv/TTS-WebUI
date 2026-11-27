import React from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { getWebuiURL, getWebuiURLWithHost } from "../data/getWebuiURL";
import { encodecDecode } from "../functions/encodecDecode";
import { saveToVoices } from "../functions/saveToVoices";
import { Button } from "./ui/button";
import {
  DownloadIcon,
  PlayIcon,
  RefreshCwIcon,
  SaveIcon,
  XIcon,
} from "lucide-react";
import { Label } from "./ui/label";
import { toLocalCacheFile } from "../types/LocalCacheFile";

export const NPZVoiceDropdown = ({
  barkGenerationParams,
  handleChange,
  name,
  label,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: (event: React.ChangeEvent<HTMLSelectElement>) => void;
  name: string;
  label: string;
}) => {
  const [options, setOptions] = React.useState<string[]>([]);

  const refreshOptions = async () => {
    const options = await reloadOldGenerationDropdown();
    setOptions(options);
  };

  React.useEffect(() => void refreshOptions(), []);

  const selected = barkGenerationParams?.[name];

  return (
    <div className="flex flex-col gap-y-2">
      <Label>{label}:</Label>
      <select
        name={name}
        id={name}
        className="cell text-black w-full"
        value={selected}
        onChange={handleChange}
      >
        <option value="">No prompt</option>
        {selected && <option value={selected}>{selected}</option>}
        {options
          .filter((option) => option !== selected)
          .map((npzFile) => (
            <option key={npzFile} value={npzFile}>
              {npzFile}
            </option>
          ))}
      </select>
      <div className="flex flex-row gap-x-2">
        <Button variant="outline" size="sm" onClick={refreshOptions}>
          <RefreshCwIcon className="w-5 h-5" />
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => handleChange({ target: { name, value: null } } as any)}
        >
          <XIcon name="x" className="w-5 h-5" />
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => window.open(getWebuiURL(selected), "_blank")}
        >
          <DownloadIcon className="w-5 h-5" />
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={async () => {
            const urlWithHost = getWebuiURLWithHost(selected);
            const x = await encodecDecode({
              npz_file: toLocalCacheFile(urlWithHost),
            });
            const audio = new Audio(x.url);
            audio.play();
          }}
        >
          Play as Audio
          <PlayIcon className="ml-2 w-5 h-5" />
        </Button>

        <Button
          variant="outline"
          size="sm"
          onClick={() => saveToVoices(selected)}
        >
          Save to Voices
          <SaveIcon className="ml-2 w-5 h-5" />
        </Button>
      </div>
    </div>
  );
};

async function reloadOldGenerationDropdown() {
  const response = await fetch("/api/gradio/reload_old_generation_dropdown", {
    method: "POST",
  });

  return await response.json();
}
