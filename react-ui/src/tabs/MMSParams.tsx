import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { Seeded } from "../types/Seeded";
import { parseFormChange } from "../data/parseFormChange";
import { generateWithMMS } from "../functions/generateWithMMS";
import { useHistory } from "../hooks/useHistory";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";
import { MetadataHeaders } from "../types/MetadataHeaders";

const MMS_ID = "MMSParams.v2";

export type MMSParams = Seeded & {
  text: string;
  language: string;
  speaking_rate: number;
  noise_scale: number;
  noise_scale_duration: number;
};

export const initialMMSParams: MMSParams = {
  seed: 0,
  use_random_seed: true,

  text: "",
  language: "eng",
  speaking_rate: 1.0,
  noise_scale: 0.667,
  noise_scale_duration: 0.8,
};

export type MMSResult = {
  audio: GradioFile;
  // metadata: MMSParams & {
  //   // _version: string;
  //   // _hash_version: string;
  //   // _type: string;
  //   // ...MMSParams,
  //   // text: string;
  //   // language: string;
  //   // speaking_rate: number;
  //   // noise_scale: number;
  //   // noise_scale_duration: number;
  // };
  metadata: MMSParams & MetadataHeaders;
};

export const sendToMMS = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    MMS_ID,
    (MMSParams: MMSParams = initialMMSParams) =>
      ({ ...MMSParams, melody } as MMSParams)
  );
  router.push("/MMS");
};

export const useMMSParams = () =>
  useLocalStorage<MMSParams>(MMS_ID, initialMMSParams);

export const useMMSResult = () =>
  useLocalStorage<MMSResult | null>(MMS_ID + ".output", null);

export const getMMSParams = (): MMSParams =>
  readLocalStorage(MMS_ID) ?? initialMMSParams;

export const useMMSPage = () => {
  const [mmsParams, setMMSParams] = useMMSParams();
  const [historyData, setHistoryData] = useHistory<MMSResult>("mms");

  const consumer = async (params: MMSParams) => {
    const data = await generateWithMMS(params);
    setMMSParams((x) => ({ ...x, seed: params.seed }));
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    favorite,
    useSeed: useSeedHelper(setMMSParams),
    useParameters: (_url: string, data?: MMSResult) => {
      const params = data?.metadata;
      if (!params) return;
      setMMSParams({
        ...mmsParams,
        ...params,
      });
    },
  };

  return {
    mmsParams,
    setMMSParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setMMSParams),
    funcs,
  };
};
