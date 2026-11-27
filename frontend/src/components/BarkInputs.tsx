import React from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { BarkVoice } from "./BarkVoice";
import { BurnInPrompt, BarkPrompt } from "./BarkPrompt";
import { NPZVoiceDropdown } from "./BarkOldGeneration";
import { PromptType, ForEachSubsequentGeneration } from "./BarkRadios";
import { SeedInput } from "./SeedInput";
import { HandleChange } from "../types/HandleChange";
import { ParameterSlider } from "./GenericSlider";

export const BarkInputs = ({
  barkGenerationParams,
  handleChange,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => (
  <div className="flex flex-col gap-y-2 p-2">
    <div className="flex flex-row gap-x-2">
      <div className="gap-y-2 flex flex-col">
        <NPZVoiceDropdown
          barkGenerationParams={barkGenerationParams}
          handleChange={handleChange}
          name="history_prompt_semantic"
          label="History Prompt Semantic"
        />
        <NPZVoiceDropdown
          barkGenerationParams={barkGenerationParams}
          handleChange={handleChange}
          name="history_prompt"
          label="History Prompt"
        />
        <BarkVoice
          barkGenerationParams={barkGenerationParams}
          handleChange={handleChange}
        />
      </div>
      <div className="flex flex-col gap-y-2">
        <div className="grid grid-cols-1 gap-4">
          <PromptType
            barkGenerationParams={barkGenerationParams}
            handleChange={handleChange}
          />
          <ForEachSubsequentGeneration
            barkGenerationParams={barkGenerationParams}
            handleChange={handleChange}
          />
        </div>
        <div className="grid grid-cols-3 cell gap-2 items-end">
          <ParameterSlider
            label="Text temperature"
            name="text_temp"
            min="0.0"
            max="1.2"
            step="0.01"
            params={barkGenerationParams}
            onChange={handleChange}
            decimals={2}
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            label="Waveform temperature"
            name="waveform_temp"
            min="0.0"
            max="1.2"
            step="0.01"
            params={barkGenerationParams}
            onChange={handleChange}
            decimals={2}
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            label="Max length (s)"
            name="max_length"
            min="0.1"
            max="18"
            step="0.1"
            params={barkGenerationParams}
            onChange={handleChange}
            decimals={1}
            orientation="vertical"
            className="h-40"
          />
        </div>
        <SeedInput params={barkGenerationParams} handleChange={handleChange} />
      </div>
    </div>
    <BurnInPrompt
      barkGenerationParams={barkGenerationParams}
      handleChange={handleChange}
    />
    <BarkPrompt
      barkGenerationParams={barkGenerationParams}
      handleChange={handleChange}
    />
  </div>
);
