import React, { useState } from "react";
import { Template } from "../../../components/Template";
import useLocalStorage from "../../../hooks/useLocalStorage";
import {
  BarkSettingsParams,
  initialBarkSettingsParams,
  barkSettingsId,
} from "../../../tabs/BarkSettingsParams";
import { GPUInfoWidget } from "../../../components/GPUInfoWidget";
import { Button } from "../../../components/ui/button";
import { SwitchWithLabel } from "../../../components/SwitchWithLabel";
import { HandleChangeEvent } from "../../../types/HandleChange";
import { Label } from "../../../components/ui/label";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from "@/components/ui/table";

const BarkSettingsPage = () => {
  const [data, setData] = useState<string>(
    "Changes will be automatically saved"
  );
  const [barkSettingsParams, setBarkSettingsParams] =
    useLocalStorage<BarkSettingsParams>(
      barkSettingsId,
      initialBarkSettingsParams
    );

  React.useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch("/api/gradio/get_config_bark");
        const data = await response.json();
        setBarkSettingsParams((x) => ({ ...x, ...data }) as BarkSettingsParams);
      } catch (error) {
        console.error("Error:", error);
      }
    };
    fetchData();
  }, [setBarkSettingsParams]);

  const handleChange = async (event: HandleChangeEvent) => {
    const { name, value, type } = event.target;

    const newBarkSettingsParams = {
      ...barkSettingsParams,
      [name]:
        type === "number" || type === "range"
          ? Number(value)
          : type === "checkbox"
            ? (event.target as HTMLInputElement).checked // type assertion
            : value,
    };

    const save_config_keys = [
      "text_use_gpu",
      "text_use_small",
      "coarse_use_gpu",
      "coarse_use_small",
      "fine_use_gpu",
      "fine_use_small",
      "codec_use_gpu",
    ];
    const isConfig = save_config_keys.includes(name);

    if (isConfig) {
      try {
        const response = await fetch("/api/gradio/save_config_bark", {
          method: "POST",
          body: JSON.stringify(
            Object.fromEntries(
              save_config_keys.map((key) => [key, newBarkSettingsParams[key]])
            )
          ),
        });
        setData(await response.json());
        setBarkSettingsParams(newBarkSettingsParams);
      } catch (error) {
        console.error("Error:", error);
      }
    }
  };

  return (
    <Template title="Bark Settings">
      <div className="flex gap-4 mt-3">
        <div className="p-4 flex flex-col gap-4 w-3/4">
          {/* <div className="flex cell gap-2">
            <input
              type="checkbox"
              name="load_models_on_startup"
              checked={barkSettingsParams.load_models_on_startup}
              onChange={handleChange}
              className="cell"
            />
            <label className="text-md">Load Bark models on startup</label>
          </div> */}
          <div className="flex cell gap-2">
            <Label className="text-md mr-auto">Text generation:</Label>
            <SwitchWithLabel
              label="Use GPU"
              name="text_use_gpu"
              value={barkSettingsParams.text_use_gpu}
              onChange={handleChange}
            />
            <SwitchWithLabel
              label="Use small model"
              name="text_use_small"
              value={barkSettingsParams.text_use_small}
              onChange={handleChange}
            />
          </div>
          <div className="flex cell gap-2">
            <Label className="text-md mr-auto">Coarse-to-fine inference:</Label>
            <SwitchWithLabel
              label="Use GPU"
              name="coarse_use_gpu"
              value={barkSettingsParams.coarse_use_gpu}
              onChange={handleChange}
            />
            <SwitchWithLabel
              label="Use small model"
              name="coarse_use_small"
              value={barkSettingsParams.coarse_use_small}
              onChange={handleChange}
            />
          </div>
          <div className="flex cell gap-2">
            <Label className="text-md mr-auto">Fine-tuning:</Label>
            <SwitchWithLabel
              label="Use GPU"
              name="fine_use_gpu"
              value={barkSettingsParams.fine_use_gpu}
              onChange={handleChange}
            />
            <SwitchWithLabel
              label="Use small model"
              name="fine_use_small"
              value={barkSettingsParams.fine_use_small}
              onChange={handleChange}
            />
          </div>
          <div className="flex cell gap-2">
            <Label className="text-md mr-auto">Codec:</Label>
            <SwitchWithLabel
              label="Use GPU"
              name="codec_use_gpu"
              value={barkSettingsParams.codec_use_gpu}
              onChange={handleChange}
            />
          </div>

          <Button
            variant="outline"
            onClick={() => {
              setBarkSettingsParams({ ...barkSettingsParams });
            }}
          >
            Reset to defaults
          </Button>
        </div>
        <div className="flex flex-col w-1/2 gap-y-4">
          <Guide />
          {/* Save Beacon */}
          <div className="cell text-xs h-20 overflow-y-scroll">{data}</div>
        </div>
      </div>
      <GPUInfoWidget
        className="flex-row w-full justify-center"
        refreshInterval={30000}
      />
    </Template>
  );
};

export default BarkSettingsPage;

function Guide() {
  return (
    <div className="flex flex-col">
      {/* Guide: */}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>VRAM</TableHead>
            <TableHead>Advice</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>&ge;&nbsp;10GB</TableCell>
            <TableCell>Use large models</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>&lt;&nbsp;10GB</TableCell>
            <TableCell>Use small models</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>&lt;&nbsp;4GB</TableCell>
            <TableCell>Use CPU offloading (requires restart)</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>&lt;&nbsp;2GB</TableCell>
            <TableCell>Use CPU offloading and only small models</TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <ul className="list-disc list-inside text-sm">
        <li>Text generation and coarse-to-fine are of similar importance.</li>
        <li>Small models might not have a perceptible difference.</li>
      </ul>
    </div>
  );
}
