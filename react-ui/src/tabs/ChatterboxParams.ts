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

export const chatterboxId = "chatterboxParams.v1";

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
}

export const defaultChatterboxParams: ChatterboxParams = {
  text: "Chatterbox is an expressive text-to-speech model with reference audio support for voice cloning.",
  exaggeration: 0.5,
  cfg_weight: 0.5,
  temperature: 0.8,
  audio_prompt_path: undefined,
  device: "auto",
  dtype: "float32",
  model_name: "just_a_placeholder",
  seed: -1,
  use_random_seed: true,
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