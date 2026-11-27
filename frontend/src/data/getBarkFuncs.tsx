import React from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { BarkResult } from "../tabs/BarkResult";
import { favorite } from "../functions/favorite";

export function getBarkFuncs(
  setBarkVoiceGenerationParams: React.Dispatch<
    React.SetStateAction<BarkGenerationParams>
  >,
  barkGenerationParams: BarkGenerationParams
) {
  const useAsHistory = (_url: string, data?: BarkResult) => {
    const npz = data?.npz;
    if (!npz) return;
    setBarkVoiceGenerationParams({
      ...barkGenerationParams,
      history_prompt: npz,
    });
  };

  const useAsHistoryPromptSemantic = (_url: string, data?: BarkResult) => {
    const npz = data?.npz;
    if (!npz) return;
    setBarkVoiceGenerationParams({
      ...barkGenerationParams,
      history_prompt_semantic: npz,
    });
  };

  const useSeed = (_url: string, data?: BarkResult) => {
    const seed_input = data?.metadata?.seed;
    if (!seed_input) return;
    setBarkVoiceGenerationParams({
      ...barkGenerationParams,
      seed: Number(seed_input),
    });
  };

  const useParametersTest = (_url: string, data?: BarkResult) => {
    const {
      prompt,
      text_temp,
      waveform_temp,
      history_prompt_npz,
      semantic_prompt,
      coarse_prompt,
    } = data?.metadata!;
    if (!prompt) return;
    setBarkVoiceGenerationParams({
      ...barkGenerationParams,
      text: prompt,
      text_temp,
      waveform_temp,
      history_prompt: history_prompt_npz,
      history_prompt_semantic: semantic_prompt,
      burn_in_prompt: coarse_prompt,
      long_prompt_radio: "Short prompt (<15s)",
      seed: Number(data?.metadata?.seed ?? "-1"),
    });
  };

  return {
    useAsHistory,
    useAsHistoryPromptSemantic,
    useSeed,
    favorite: favorite,
    useParametersTest,
  };
}
