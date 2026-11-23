import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { TortoiseResult } from "./TortoiseResult";
import { Seeded } from "../types/Seeded";
import { parseFormChange } from "../data/parseFormChange";
import { favorite } from "../functions/favorite";
import { generateWithTortoise } from "../functions/generateWithTortoise";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { useHistory } from "../hooks/useHistory";

export type TortoiseGenerationParams = Seeded & {
  text: string; // string  in 'Prompt' Textbox component
  speaker: string; // string in 'parameter_2502' Dropdown component
  preset: string; // string (Option from: ['ultra_fast', 'fast', 'standard', 'high_quality']) in 'parameter_2507' Dropdown component
  cvvp_amount: number; // number (numeric value between 0.0 and 1.0) in 'CVVP Amount' Slider component
  split_prompt: boolean; // boolean  in 'Split prompt by lines' Checkbox component
  samples: number; // number (numeric value between 4 and 256) in 'Samples' Slider component
  diffusion_iterations: number; // number (numeric value between 4 and 400) in 'Diffusion Iterations' Slider component
  temperature: number; // number (numeric value between 0.0 and 1.0) in 'Temperature' Slider component
  length_penalty: number; // number (numeric value between 0.0 and 10.0) in 'Length Penalty' Slider component
  repetition_penalty: number; // number (numeric value between 0.0 and 10.0) in 'Repetition Penalty' Slider component
  top_p: number; // number (numeric value between 0.0 and 1.0) in 'Top P' Slider component
  max_mel_tokens: number; // number (numeric value between 10 and 600) in 'Max Mel Tokens' Slider component
  cond_free: boolean; // boolean  in 'Cond Free' Checkbox component
  cond_free_k: number; // number (numeric value between 0 and 10) in 'Cond Free K' Slider component
  diffusion_temperature: number; // number (numeric value between 0.0 and 1.0) in 'Temperature' Slider component
  generation_name: string; // string  in 'Generation Name' Textbox component

  model: string; // string (Option from: ['Default']) in 'parameter_2488' Dropdown component
  kv_cache: boolean; // boolean  in 'parameter_2493' Checkbox component
  use_deepspeed: boolean; // boolean  in 'parameter_2494' Checkbox component
  half: boolean; // boolean  in 'parameter_2495' Checkbox component
  use_basic_cleaners: boolean; // boolean  in 'parameter_2497' Checkbox component
  tokenizer: string; // string

  candidates: number;
};

export const initialState: TortoiseGenerationParams = {
  seed: 0,
  use_random_seed: true,

  //   autoregressive
  samples: 16,
  temperature: 0.8,
  length_penalty: 1.0,
  repetition_penalty: 2.0,
  top_p: 0.8,
  max_mel_tokens: 500,

  //  diffusion
  diffusion_iterations: 30,
  cond_free: false,
  cond_free_k: 2,
  diffusion_temperature: 1.0,

  model: "Default", // string (Option from: ['Default']) in 'parameter_2488' Dropdown component

  text: "Howdy!", // string  in 'Prompt' Textbox component
  speaker: "random", // string in 'parameter_2502' Dropdown component
  preset: "ultra_fast", // string (Option from: ['ultra_fast', 'fast', 'standard', 'high_quality']) in 'parameter_2507' Dropdown component
  cvvp_amount: 0, // number (numeric value between 0.0 and 1.0) in 'CVVP Amount' Slider component
  split_prompt: true, // boolean  in 'Split prompt by lines' Checkbox component
  generation_name: "", // string  in 'Generation Name' Textbox component

  kv_cache: false, // boolean  in 'parameter_2493' Checkbox component
  use_deepspeed: false, // boolean  in 'parameter_2494' Checkbox component
  half: false, // boolean  in 'parameter_2495' Checkbox component
  use_basic_cleaners: false, // boolean  in 'parameter_2497' Checkbox component
  tokenizer: "", // string

  candidates: 1,
};

export const tortoiseGenerationId = "tortoise_generation-tab.v2";

export const useTortoiseGenerationParams = () =>
  useLocalStorage<TortoiseGenerationParams>(tortoiseGenerationId, initialState);

export const useTortoiseResult = () =>
  useLocalStorage<TortoiseResult | null>(
    tortoiseGenerationId + ".output",
    null
  );

export const getTortoiseGenerationParams = (): TortoiseGenerationParams =>
  readLocalStorage(tortoiseGenerationId) ?? initialState;

export const useTortoisePage = () => {
  const [tortoiseGenerationParams, setTortoiseGenerationParams] =
    useTortoiseGenerationParams();

  const [historyData, setHistoryData] = useHistory<TortoiseResult>("tortoise.v3");

  const consumer = async (params: TortoiseGenerationParams) => {
    const results = await generateWithTortoise(params);
    if (params.use_random_seed)
      setTortoiseGenerationParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [...results, ...x]);
    return results;
  };

  const funcs = {
    favorite: async (_url: string, data?: TortoiseResult) => {
      const folder_root = data?.bundle_name;
      favorite("", { folder_root });
    },
    useSeed: useSeedHelper(setTortoiseGenerationParams),
    // useParameters: (_url: string, data?: TortoiseResult) => {
    //   const params = data?.metadata;
    //   if (!params) return;
    //   setTortoiseGenerationParams({
    //     ...tortoiseGenerationParams,
    //     ...params,
    //     seed: Number(params.seed),
    //   });
    // },
  };

  return {
    tortoiseGenerationParams,
    setTortoiseGenerationParams,
    historyData,
    setHistoryData,
    consumer,
    funcs,
    handleChange: parseFormChange(setTortoiseGenerationParams),
  };
};
