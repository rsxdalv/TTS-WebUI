import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { Seeded } from "../types/Seeded";
import { useHistory } from "../hooks/useHistory";
import { generateWithMusicgen } from "../functions/generateWithMusicgen";
import { parseFormChange } from "../data/parseFormChange";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";

export const musicgenId = "musicgenParams.v3";

export type MusicgenParams = Seeded & {
  text: string;
  melody?: string;
  model_name: string;
  duration: number;
  topk: number;
  topp: number;
  temperature: number;
  cfg_coef: number;
  use_multi_band_diffusion: boolean;
};

export const initialMusicgenParams: MusicgenParams = {
  seed: 0,
  use_random_seed: true,

  text: "lofi hip hop beats to relax/study to",
  melody: undefined,
  model_name: "facebook/musicgen-small",
  duration: 1,
  topk: 250,
  topp: 0,
  temperature: 1.0,
  cfg_coef: 3.0,
  use_multi_band_diffusion: false,
};

export type MusicgenResult = {
  audio: GradioFile;
  folder_root: string;
  metadata: {
    _version: string;
    _hash_version: string;
    _type: string;
    _audiocraft_version: string;
    models: {};
    prompt: string;
    hash: string;
    date: string;
    melody?: any;
    text: string;
    model: string;
    duration: number;
    topk: number;
    topp: number;
    temperature: number;
    cfg_coef: number;
    seed: string;
    use_multi_band_diffusion: boolean;
  };
};

export const sendToMusicgen = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    musicgenId,
    (musicgenParams: MusicgenParams = initialMusicgenParams) =>
      ({ ...musicgenParams, melody } as MusicgenParams)
  );
  router.push("/musicgen");
};

export const useMusicgenParams = () =>
  useLocalStorage<MusicgenParams>(musicgenId, initialMusicgenParams);

export const useMusicgenResult = () =>
  useLocalStorage<MusicgenResult | null>(musicgenId + ".output", null);

export const getMusicgenParams = (): MusicgenParams =>
  readLocalStorage(musicgenId) ?? initialMusicgenParams;

export const useMusicgenPage = () => {
  const [musicgenParams, setMusicgenParams] = useMusicgenParams();

  const [historyData, setHistoryData] = useHistory<MusicgenResult>("musicgen");

  const consumer = async (params: MusicgenParams) => {
    const data = await generateWithMusicgen(params);
    setMusicgenParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    useAsMelody: (melody?: string, metadata?: MusicgenResult) => {
      if (!melody) return;
      setMusicgenParams({
        ...musicgenParams,
        melody,
      });
    },
    favorite,
    useSeed: useSeedHelper(setMusicgenParams),
    useParameters: (_url: string, data?: MusicgenResult) => {
      const params = data?.metadata;
      if (!params) return;
      setMusicgenParams({
        ...musicgenParams,
        ...params,
        seed: Number(params.seed),
        model_name: params.model || "facebook/musicgen-small",
      });
    },
  };

  return {
    musicgenParams,
    setMusicgenParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setMusicgenParams),
    funcs,
  };
};
