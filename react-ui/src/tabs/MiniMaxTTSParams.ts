import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { favorite } from "../functions/favorite";
import { GradioFile } from "../types/GradioFile";
import { MetadataHeaders } from "../types/MetadataHeaders";
import { generateWithMiniMaxTTS } from "../functions/generateWithMiniMaxTTS";

export const minimaxTTSId = "minimaxTTSParams.v1";

export interface MiniMaxTTSParams {
  [key: string]: string;
  text: string;
  model: string;
  voice_id: string;
  api_key: string;
}

export const defaultMiniMaxTTSParams: MiniMaxTTSParams = {
  text: "Hello! Welcome to MiniMax Cloud Text-to-Speech. This is a demonstration of high-quality speech synthesis.",
  model: "speech-2.8-hd",
  voice_id: "English_Graceful_Lady",
  api_key: "",
};

export type MiniMaxTTSResult = {
  audio: GradioFile;
  metadata: MiniMaxTTSParams & MetadataHeaders;
};

export function useMiniMaxTTSPage() {
  const [minimaxTTSParams, setMiniMaxTTSParams] =
    useLocalStorage<MiniMaxTTSParams>(minimaxTTSId, defaultMiniMaxTTSParams);

  const [historyData, setHistoryData] =
    useHistory<MiniMaxTTSResult>("minimax_cloud_tts");

  const consumer = async (params: MiniMaxTTSParams) => {
    const result = await generateWithMiniMaxTTS(params);
    setHistoryData((prev) => [result, ...prev]);
    return result;
  };

  const funcs = {
    favorite: (metadata: any) => favorite(metadata),
    useParameters: (_url: string, data?: MiniMaxTTSResult) => {
      const params = data?.metadata;
      if (!params) return;
      setMiniMaxTTSParams({
        ...minimaxTTSParams,
        ...params,
      });
    },
  };

  const handleChange = parseFormChange(setMiniMaxTTSParams);

  const resetParams = () => {
    setMiniMaxTTSParams({
      ...defaultMiniMaxTTSParams,
      text: "",
    });
  };

  return {
    historyData,
    setHistoryData,
    minimaxTTSParams,
    setMiniMaxTTSParams,
    consumer,
    handleChange,
    funcs,
    resetParams,
  };
}
