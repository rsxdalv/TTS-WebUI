import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";
import { Seeded } from "../types/Seeded";
import { GradioFile } from "../types/GradioFile";
import { generateWithStableAudio } from "../functions/generateWithStableAudio";

export const stableAudioId = "stableAudioParams.v3";

export type StableAudioParams = Seeded & {
  text: string;
  negative_prompt: string;
  seconds_start: number;
  seconds_total: number;
  cfg_scale: number;
  steps: number;
  preview_every: number;
  sampler_type: string;
  sigma_min: number;
  sigma_max: number;
  cfg_rescale: number;
  use_init: boolean;
  init_audio?: string; // GradioFile;
  init_noise_level: number;
};

export const initialStableAudioParams: StableAudioParams = {
  seed: 0,
  use_random_seed: true,

  text: "lofi hip hop beats to relax/study to",
  negative_prompt: "",
  seconds_start: 0,
  seconds_total: 60,
  cfg_scale: 7,
  steps: 100,
  preview_every: 0,
  sampler_type: "dpmpp-3m-sde",
  sigma_min: 0.03,
  sigma_max: 500,
  cfg_rescale: 0,
  use_init: false,
  init_audio: undefined,
  init_noise_level: 0.1,
};

export type StableAudioResult = {
  audio: GradioFile;
  folder_root: string;
  metadata: {
    _version: string;
    _hash_version: string;
    _type: string;
    prompt: string;
    negative_prompt: string;
    seconds_start: number;
    seconds_total: number;
    cfg_scale: number;
    steps: number;
    preview_every: number;
    date: string;
    seed: string;
  };
};

export function useStableAudioPage() {
  const [stableAudioParams, setStableAudioParams] = useLocalStorage(
    stableAudioId,
    initialStableAudioParams
  );

  const [historyData, setHistoryData] =
    useHistory<StableAudioResult>("stableAudio");

  const consumer = async (params: StableAudioParams) => {
    const result = await generateWithStableAudio(params);
    setHistoryData((prev) => [result, ...prev]);
    return result;
  };

  const funcs = {
    favorite: (metadata: any) => favorite(metadata),
    useSeed: useSeedHelper(setStableAudioParams),
    useParameters: (_url: string, data?: StableAudioResult) => {
      const params = data?.metadata;
      if (!params) return;
      setStableAudioParams({
        ...stableAudioParams,
        ...params,
        seed: Number(params.seed),
      });
    },
  };

  return {
    stableAudioParams,
    setStableAudioParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setStableAudioParams),
    funcs,
  };
}
