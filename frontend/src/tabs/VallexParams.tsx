import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { Seeded } from "../types/Seeded";
import { useHistory } from "../hooks/useHistory";
import { generateWithVallex } from "../functions/generateWithVallex";
import { parseFormChange } from "../data/parseFormChange";

const vallex_ID = "vallexParams.v2";

export type VallexParams = Seeded & {
  text: string;
  prompt: string;
  language: string;
  accent: string;
  mode: string;
};

export const initialVallexParams: VallexParams = {
  seed: 0,
  use_random_seed: true,

  text: "",
  prompt: "",
  language: "Mix",
  accent: "no-accent",
  mode: "short",
};

export type VallexResult = {
  audio: GradioFile;
  metadata: {
    _version: string;
    _hash_version: string;
    _type: string;
    text: string;
    prompt: string;
    language: string;
    accent: string;
    mode: string;
  };
};

export const sendToVallex = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    vallex_ID,
    (vallexParams: VallexParams = initialVallexParams) =>
      ({ ...vallexParams, melody } as VallexParams)
  );
  router.push("/vallex");
};

export const useVallexParams = () =>
  useLocalStorage<VallexParams>(vallex_ID, initialVallexParams);

export const useVallexResult = () =>
  useLocalStorage<VallexResult | null>(vallex_ID + ".output", null);

export const getVallexParams = (): VallexParams =>
  readLocalStorage(vallex_ID) ?? initialVallexParams;

export const useVallexPage = () => {
  const [vallexParams, setVallexParams] = useVallexParams();

  const [historyData, setHistoryData] = useHistory<VallexResult>("vallex");

  const consumer = async (params: VallexParams) => {
    const data = await generateWithVallex(params);
    setVallexParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    useParameters: (_url: string, data?: VallexResult) => {
      const params = data?.metadata;
      if (!params) return;
      setVallexParams({
        ...vallexParams,
        ...params,
      });
    },
  };

  return {
    vallexParams,
    setVallexParams,
    historyData,
    setHistoryData,
    consumer,
    funcs,
    handleChange: parseFormChange(setVallexParams),
  };
};
