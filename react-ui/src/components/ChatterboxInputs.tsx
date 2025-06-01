import React from "react";
import { HandleChange } from "../types/HandleChange";
import { ChatterboxParams } from "../tabs/ChatterboxParams";
import { PromptTextArea } from "./PromptTextArea";
import { ParameterSlider } from "./GenericSlider";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { SingleFileUpload } from "./SingleFileUpload";
import { toLocalCacheFile } from "../types/LocalCacheFile";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import { UnloadModelButton } from "./component/ModelDropdown";
import { AudioInput } from "./AudioComponents";

// Type alias for components that need basic params
type ChatterboxBasicParams = Omit<ChatterboxParams, "audio_prompt_path"> & {
  [key: string]: string | number | boolean;
};

export const ChatterboxInputs = ({
  chatterboxParams,
  handleChange,
  setChatterboxParams,
}: {
  chatterboxParams: ChatterboxParams;
  handleChange: HandleChange;
  setChatterboxParams: React.Dispatch<React.SetStateAction<ChatterboxParams>>;
}) => (
  <div className="flex flex-col gap-4 w-full justify-center">
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Text Input</CardTitle>
        <CardDescription>
          Enter the text you want to convert to speech using Chatterbox TTS.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <PromptTextArea
          params={chatterboxParams as ChatterboxBasicParams}
          handleChange={handleChange}
          label=""
          name="text"
          rows={8}
          placeholder="Enter text to synthesize..."
        />
      </CardContent>
    </Card>
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Voice Parameters</CardTitle>
        <CardDescription>
          Adjust voice characteristics and generation settings.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <ParameterSlider
          params={chatterboxParams as ChatterboxBasicParams}
          onChange={handleChange}
          label="Exaggeration"
          name="exaggeration"
          min="0"
          max="2"
          step="0.05"
          decimals={2}
        />

        <ParameterSlider
          params={chatterboxParams as ChatterboxBasicParams}
          onChange={handleChange}
          label="CFG Weight/Pace"
          name="cfg_weight"
          min="0.2"
          max="1"
          step="0.05"
          decimals={2}
        />

        <ParameterSlider
          params={chatterboxParams as ChatterboxBasicParams}
          onChange={handleChange}
          label="Temperature"
          name="temperature"
          min="0.05"
          max="5"
          step="0.05"
          decimals={2}
        />

        <div className="pt-2">
          <AudioInput
            url={chatterboxParams.audio_prompt_path?.path}
            label="Reference Audio"
            callback={(file) => {
              setChatterboxParams({
                ...chatterboxParams,
                audio_prompt_path: toLocalCacheFile(file),
              });
            }}
            filter={["sendToChatterbox"]}
          />
        </div>

        <div className="flex items-center gap-4 pt-2">
          <RadioWithLabel
            label="Device"
            name="device"
            inline
            value={chatterboxParams.device}
            onChange={handleChange}
            options={[
              ["Auto", "auto"],
              ["CUDA", "cuda"],
              ["MPS", "mps"],
              ["CPU", "cpu"],
            ].map(([text, value]) => ({
              label: text,
              value,
            }))}
          />
          <UnloadModelButton
            onUnload={() =>
              fetch("/api/gradio/chatterbox_unload_model", { method: "POST" })
            }
          />
        </div>
      </CardContent>
    </Card>
  </div>
);
