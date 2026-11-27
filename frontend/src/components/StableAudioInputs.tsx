import React from "react";
import { StableAudioParams } from "../tabs/StableAudioParams";
import { PromptTextArea } from "./PromptTextArea";
import { HandleChange } from "../types/HandleChange";
import { ParameterSlider } from "./GenericSlider";
import { ModelDropdown } from "./component/ModelDropdown";

const StableAudioModels = ({
  params,
  onChange,
  label,
  name,
}: {
  params: StableAudioParams;
  onChange: HandleChange;
  label: string;
  name: string;
}) => (
  <div />
  // <ModelDropdown
  //   name={name}
  //   label={label}
  //   options={params.model_name}
  //   value={params.model_name}
  //   onChange={onChange}
  // />
);

interface StableAudioInputsProps {
  stableAudioParams: StableAudioParams;
  handleChange: HandleChange;
  setStableAudioParams: React.Dispatch<React.SetStateAction<StableAudioParams>>;
}

export const StableAudioInputs: React.FC<StableAudioInputsProps> = ({
  stableAudioParams,
  handleChange,
  setStableAudioParams,
}) => {
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="flex flex-col gap-2">
        <PromptTextArea
          params={stableAudioParams}
          handleChange={handleChange}
          label="Text"
          name="text"
        />
        <PromptTextArea
          params={stableAudioParams}
          handleChange={handleChange}
          label="Negative Prompt"
          name="negative_prompt"
        />
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="flex flex-col gap-2 cell">
          <ParameterSlider
            params={stableAudioParams}
            onChange={handleChange}
            label="Seconds Start"
            name="seconds_start"
            min="0"
            max="512"
            step="1"
          />
          <ParameterSlider
            params={stableAudioParams}
            onChange={handleChange}
            label="Seconds Total"
            name="seconds_total"
            min="0"
            max="512"
            step="1"
          />
        </div>
        <div className="flex gap-2 cell justify-center">
          <ParameterSlider
            params={stableAudioParams}
            onChange={handleChange}
            label="CFG Scale"
            name="cfg_scale"
            min="0"
            max="50"
            step="0.01"
            decimals={2}
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            params={stableAudioParams}
            onChange={handleChange}
            label="Steps"
            name="steps"
            min="0"
            max="500"
            step="1"
            orientation="vertical"
            className="h-40"
          />
        </div>
        <div className="hidden">
          <ParameterSlider
            params={stableAudioParams}
            onChange={handleChange}
            label="Preview Every"
            name="preview_every"
            min="0"
            max="100"
            step="1"
            orientation="vertical"
            className="h-40"
            disabled
          />
        </div>
        <StableAudioModels
          params={stableAudioParams}
          onChange={handleChange}
          label="Model"
          name="model_name"
        />
      </div>
    </div>
  );
};
