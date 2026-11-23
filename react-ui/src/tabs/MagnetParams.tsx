import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { parseFormChange } from "../data/parseFormChange";
import { useHistory } from "../hooks/useHistory";
import { generateWithMagnet } from "../functions/generateWithMagnet";
import { favorite } from "../functions/favorite";
import { Seeded } from "../types/Seeded";
import { useSeedHelper } from "../functions/results/useSeedHelper";

export const magnetId = "magnetParams.v4";

export type MagnetParams = Seeded & {
  model_name: string;
  text: string;
  use_sampling: boolean;
  top_k: number;
  top_p: number;
  temperature: number;
  max_cfg_coef: number;
  min_cfg_coef: number;
  decoding_steps_1: number;
  decoding_steps_2: number;
  decoding_steps_3: number;
  decoding_steps_4: number;
  span_arrangement: string;
};

export const initialMagnetParams: MagnetParams = {
  model_name: "facebook/magnet-small-10secs",
  text: "",
  seed: 0,
  use_sampling: true,
  top_k: 0,
  top_p: 0.9,
  temperature: 1,
  max_cfg_coef: 20.0,
  min_cfg_coef: 1.0,
  decoding_steps_1: 80,
  decoding_steps_2: 40,
  decoding_steps_3: 40,
  decoding_steps_4: 40,
  span_arrangement: "nonoverlap",
  use_random_seed: true,
};

export type MagnetResult = {
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
    model: string;
    text: string;
    seed: string;
    use_sampling: boolean;
    top_k: number;
    top_p: number;
    temperature: number;
    max_cfg_coef: number;
    min_cfg_coef: number;
    decoding_steps: number[];
    span_arrangement: string;
  };
};

export const sendToMagnet = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    magnetId,
    (magnetParams: MagnetParams = initialMagnetParams) =>
      ({ ...magnetParams, melody } as MagnetParams)
  );
  router.push("/magnet");
};

export const useMagnetParams = () =>
  useLocalStorage<MagnetParams>(magnetId, initialMagnetParams);

export const useMagnetResult = () =>
  useLocalStorage<MagnetResult | null>(magnetId + ".output", null);

export const getMagnetParams = (): MagnetParams =>
  readLocalStorage(magnetId) ?? initialMagnetParams;

export const useMagnetPage = () => {
  const [magnetParams, setMagnetParams] = useMagnetParams();

  const [historyData, setHistoryData] = useHistory<MagnetResult>("magnet.v2");

  const consumer = async (params: MagnetParams) => {
    const data = await generateWithMagnet(params);
    setMagnetParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    favorite,
    useSeed: useSeedHelper(setMagnetParams),
    useParameters: (_url: string, data?: MagnetResult) => {
      const params = data?.metadata;
      if (!params) return;
      setMagnetParams({
        ...magnetParams,
        ...params,
        seed: Number(params.seed),
        model_name: params.model || initialMagnetParams.model_name,
        decoding_steps_1: params.decoding_steps[0],
        decoding_steps_2: params.decoding_steps[1],
        decoding_steps_3: params.decoding_steps[2],
        decoding_steps_4: params.decoding_steps[3],
      });
    },
  };

  return {
    magnetParams,
    setMagnetParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setMagnetParams),
    funcs,
  };
};
