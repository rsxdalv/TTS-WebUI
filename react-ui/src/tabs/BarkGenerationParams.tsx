import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "next/router";
import { parseFormChange } from "../data/parseFormChange";
import { generateWithBark } from "../functions/generateWithBark";
import { useHistory } from "../hooks/useHistory";
import { BarkResult } from "./BarkResult";
import { getBarkFuncs } from "../data/getBarkFuncs";
import { Seeded } from "../types/Seeded";

export type BarkGenerationParams = Seeded & {
  burn_in_prompt: string; // string  in 'Burn In Prompt (Optional)' Textbox component
  text: string; // string  in 'Prompt' Textbox component
  text_temp: number; // number (numeric value between 0.0 and 1.2) in 'Text temperature' Slider component
  waveform_temp: number; // number (numeric value between 0.0 and 1.2) in 'Waveform temperature' Slider component
  long_prompt_radio: string; // string  in 'Prompt type' Radio component
  long_prompt_history_radio: string; // string  in 'For each subsequent generation:' Radio component
  history_prompt: string; // string
  history_prompt_semantic: string; // string
  max_length: number; // number  in 'Max generation duration (s)' Number component
};

export const initialState: BarkGenerationParams = {
  seed: 0,
  use_random_seed: true,

  burn_in_prompt: "",
  text: "",
  text_temp: 0.7,
  waveform_temp: 0.7,
  long_prompt_radio: "Short prompt (<15s)",
  long_prompt_history_radio: "Use old generation as history",
  history_prompt: "",
  history_prompt_semantic: "",
  max_length: 15,
};

export const barkGenerationId = "bark_generation-tab.v5";

export const sendToBarkAsVoice = (voice?: string) => {
  if (!voice) return;
  updateLocalStorageWithFunction(
    barkGenerationId,
    (
      barkParams: BarkGenerationParams = initialState
    ): BarkGenerationParams => ({
      ...barkParams,
      history_prompt: voice,
      history_prompt_semantic: voice,
    })
  );
  router.push("/text-to-speech/bark/generation");
};

export const useBarkGenerationParams = () =>
  useLocalStorage<BarkGenerationParams>(barkGenerationId, initialState);

export const getBarkGenerationParams = (): BarkGenerationParams =>
  readLocalStorage(barkGenerationId) ?? initialState;

export const useBarkPage = () => {
  const [barkGenerationParams, setBarkGenerationParams] =
    useBarkGenerationParams();

  const [historyData, setHistoryData] = useHistory<BarkResult>("bark");

  async function consumer(params: BarkGenerationParams) {
    const result = await generateWithBark(params);
    setHistoryData((historyData) => [result, ...historyData]);
    setBarkGenerationParams((x) => ({ ...x, seed: params.seed }));
  }

  return {
    barkGenerationParams,
    setBarkGenerationParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setBarkGenerationParams),
    funcs: getBarkFuncs(setBarkGenerationParams, barkGenerationParams),
  };
};
