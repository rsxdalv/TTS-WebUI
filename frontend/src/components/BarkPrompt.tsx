import React from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";

export const BarkPrompt = ({
  barkGenerationParams,
  handleChange,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => (
  <PromptTextArea
    params={barkGenerationParams}
    handleChange={handleChange}
    label="Prompt"
    name="text"
  />
);

export const BurnInPrompt = ({
  barkGenerationParams,
  handleChange,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => (
  <PromptTextArea
    params={barkGenerationParams}
    handleChange={handleChange}
    label="Burn In Prompt (Optional)"
    name="burn_in_prompt"
  />
);
