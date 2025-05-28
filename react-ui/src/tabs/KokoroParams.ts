import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";
import { Seeded } from "../types/Seeded";
import { GradioFile } from "../types/GradioFile";
import router from "next/router";
import { MetadataHeaders } from "../types/MetadataHeaders";
import { updateLocalStorageWithFunction } from "../data/updateLocalStorageWithFunction";

export const kokoroId = "kokoroParams.v4";

export const sendToKokoro = (audio?: string) => {
  if (!audio) return;
  // For Kokoro, we don't have a direct audio input parameter
  // Just navigate to the page
  router.push("/text-to-speech/kokoro");
};

export type KokoroParams = Seeded & {
  text: string;
  voice: string;
  speed: number;
  model_name: string;
  use_gpu: boolean;
};

export const initialKokoroParams: KokoroParams = {
  seed: 0,
  use_random_seed: true,

  text: "Kokoro is an open-weight TTS model with 82 million parameters. Despite its lightweight architecture, it delivers comparable quality to larger models while being significantly faster and more cost-efficient.",
  voice: "af_heart",
  speed: 1.0,
  model_name: "hexgrad/Kokoro-82M",
  use_gpu: true,
};

export type KokoroResult = {
  audio: GradioFile;
  metadata: KokoroParams & MetadataHeaders;
};

export function useKokoroPage() {
  const [kokoroParams, setKokoroParams] = useLocalStorage(
    kokoroId,
    initialKokoroParams
  );

  const [historyData, setHistoryData] = useHistory<KokoroResult>("kokoro");

  const consumer = async (params: KokoroParams) => {
    const result = await generateWithKokoro(params);
    setHistoryData((prev) => [result, ...prev]);
    return result;
  };

  const funcs = {
    favorite: (metadata: any) => favorite(metadata),
    useSeed: useSeedHelper(setKokoroParams),
    useParameters: (_url: string, data?: KokoroResult) => {
      const params = data?.metadata;
      if (!params) return;
      setKokoroParams({
        ...kokoroParams,
        ...params,
        seed: Number(params.seed),
      });
    },
  };

  return {
    kokoroParams,
    setKokoroParams,
    resetParams: () => setKokoroParams({ ...kokoroParams, ...initialKokoroParams }),
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setKokoroParams),
    funcs,
  };
}

// This function will be implemented in a separate file
import { generateWithKokoro } from "../functions/generateWithKokoro";
