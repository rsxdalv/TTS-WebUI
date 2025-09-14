import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";
import { Seeded } from "../types/Seeded";
import { GradioFile } from "../types/GradioFile";
import { LocalCacheFile } from "../types/LocalCacheFile";
import router from "next/router";
import { MetadataHeaders } from "../types/MetadataHeaders";
import { generateWithChatterbox } from "../functions/generateWithChatterbox";

export const chatterboxId = "chatterboxParams.v3";

export const sendToChatterbox = (audio?: string) => {
  if (!audio) return;
  router.push("/text-to-speech/chatterbox");
};

export interface ChatterboxParams extends Seeded {
  [key: string]: string | number | boolean | LocalCacheFile | undefined;
  text: string;
  exaggeration: number;
  cfg_weight: number;
  temperature: number;
  audio_prompt_path?: LocalCacheFile;
  device: string;
  dtype: string;
  model_name: string;
  chunked: boolean;
  halve_first_chunk: boolean;
  desired_length: number;
  max_length: number;
  cpu_offload: boolean;
  initial_forward_pass_backend?: string;
  generate_token_backend?: string;
  max_new_tokens: number;
  max_cache_len: number;
  language_id?: string;
}

export const defaultChatterboxParams: ChatterboxParams = {
  text: "Chatterbox is an expressive text-to-speech model with reference audio support for voice cloning.",
  exaggeration: 0.5,
  cfg_weight: 0.5,
  temperature: 0.8,
  audio_prompt_path: undefined,
  device: "auto",
  dtype: "bfloat16",
  model_name: "just_a_placeholder",
  seed: -1,
  use_random_seed: true,
  chunked: false,
  halve_first_chunk: false,
  desired_length: 200,
  max_length: 300,
  cpu_offload: false,
  initial_forward_pass_backend: "eager",
  generate_token_backend: "cudagraphs-manual",
  max_new_tokens: 1000,
  max_cache_len: 1500,
  language_id: "en",
};

export type ChatterboxResult = {
  audio: GradioFile;
  metadata: ChatterboxParams & MetadataHeaders;
};

export const useChatterboxParams = () => {
  const [chatterboxParams, setChatterboxParams] = useLocalStorage<ChatterboxParams>(
    chatterboxId,
    defaultChatterboxParams
  );

  const handleChange = parseFormChange(setChatterboxParams);

  return {
    chatterboxParams,
    setChatterboxParams,
    handleChange,
  };
};

export function useChatterboxPage() {
  const [chatterboxParams, setChatterboxParams] = useLocalStorage(
    chatterboxId,
    defaultChatterboxParams
  );

  const [historyData, setHistoryData] = useHistory<ChatterboxResult>("chatterbox");

  const consumer = async (params: ChatterboxParams) => {
    const result = await generateWithChatterbox(params);
    setHistoryData((prev) => [result, ...prev]);
    return result;
  };

  const funcs = {
    favorite: (metadata: any) => favorite(metadata),
    useSeed: useSeedHelper(setChatterboxParams),
    useParameters: (_url: string, data?: ChatterboxResult) => {
      const params = data?.metadata;
      if (!params) return;
      setChatterboxParams({
        ...chatterboxParams,
        ...params,
        seed: Number(params.seed),
      });
    },
  };

  const handleChange = parseFormChange(setChatterboxParams);

  const resetParams = () => {
    setChatterboxParams({
      ...defaultChatterboxParams,
      text: "",
    });
  };

  return {
    historyData,
    setHistoryData,
    chatterboxParams,
    setChatterboxParams,
    consumer,
    handleChange,
    funcs,
    resetParams,
  };
}