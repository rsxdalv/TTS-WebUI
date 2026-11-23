import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { parseFormChange } from "../data/parseFormChange";
import { favorite } from "../functions/favorite";
import { generateWithMaha } from "../functions/generateWithMaha";
import { useHistory } from "../hooks/useHistory";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { Seeded } from "../types/Seeded";
import { MetadataHeaders } from "../types/MetadataHeaders";

const mahaId = "mahaParams.v4";

export type MahaParams = Seeded & {
  text: string;
  model_name: string;
  text_language: string;
  speaker_name: string;
  device: string;
};

export const initialMahaParams: MahaParams = {
  seed: 0,
  use_random_seed: true,

  text: "",
  model_name: "Smolie-in",
  text_language: "english",
  speaker_name: "",
  device: "auto",
};

export type MahaResult = {
  audio: GradioFile;
  // metadata: {
  //   _version: string;
  //   _hash_version: string;
  //   _type: string;
  //   text: string;
  //   model_language: string;
  //   text_language: string;
  //   speaker_name: string;
  //   seed: string;
  // };
  metadata: MahaParams & MetadataHeaders;
};

export const sendToMaha = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    mahaId,
    (mahaParams: MahaParams = initialMahaParams) =>
      ({ ...mahaParams, melody } as MahaParams)
  );
  router.push("/maha");
};

export const useMahaParams = () =>
  useLocalStorage<MahaParams>(mahaId, initialMahaParams);

export const useMahaResult = () =>
  useLocalStorage<MahaResult | null>(mahaId + ".output", null);

export const getMahaParams = (): MahaParams =>
  readLocalStorage(mahaId) ?? initialMahaParams;

export const useMahaPage = () => {
  const [mahaParams, setMahaParams] = useMahaParams();

  const [historyData, setHistoryData] = useHistory<MahaResult>("maha");

  const consumer = async (params: MahaParams) => {
    const data = await generateWithMaha(params);
    setMahaParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    favorite,
    useSeed: useSeedHelper(setMahaParams),
    useParameters: (_url: string, data?: MahaResult) => {
      const params = data?.metadata;
      if (!params) return;
      setMahaParams({
        ...mahaParams,
        ...params,
        seed: Number(params.seed),
      });
    },
  };

  return {
    mahaParams,
    setMahaParams,
    resetParams: () => setMahaParams({ ...mahaParams, ...initialMahaParams }),
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setMahaParams),
    funcs,
  };
};
