import React from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { HandleChange } from "../types/HandleChange";
import { RadioWithLabel } from "./component/RadioWithLabel";

export const ForEachSubsequentGeneration = ({
  barkGenerationParams,
  handleChange,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => (
  <RadioWithLabel
    label="For each subsequent generation"
    name="long_prompt_history_radio"
    inline
    className="flex-col items-start"
    value={barkGenerationParams.long_prompt_history_radio}
    onChange={handleChange}
    options={[
      "Use old generation as history",
      "or Use history prompt setting",
      "or Clear history",
    ].map((long_prompt_history_radio) => ({
      label: long_prompt_history_radio,
      value: long_prompt_history_radio,
    }))}
  />
);

export const PromptType = ({
  barkGenerationParams,
  handleChange,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => (
  <RadioWithLabel
    label="Prompt type"
    name="long_prompt_radio"
    inline
    className="flex-col items-start"
    value={barkGenerationParams.long_prompt_radio}
    onChange={handleChange}
    options={["Short prompt (<15s)", "Split prompt by lines"].map(
      (long_prompt_radio) => ({
        label: long_prompt_radio,
        value: long_prompt_radio,
      })
    )}
  />
);
