import React from "react";
import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { favorite } from "../functions/favorite";
import { GradioFile } from "../types/GradioFile";
import { MetadataHeaders } from "../types/MetadataHeaders";
import { generateWithMiniMaxTTS } from "../functions/generateWithMiniMaxTTS";

export const minimaxTTSId = "minimaxTTSParams.v1";

export interface MiniMaxTTSParams {
  text: string;
  model: string;
  voice_id: string;
  seed: number;
  use_random_seed: boolean;
  [key: string]: string | number | boolean;
}

export const defaultMiniMaxTTSParams: MiniMaxTTSParams = {
  text: "Hello! Welcome to MiniMax Cloud Text-to-Speech. This is a demonstration of high-quality speech synthesis.",
  model: "speech-2.8-hd",
  voice_id: "English_Graceful_Lady",
  seed: 0,
  use_random_seed: true,
};

export type MiniMaxTTSResult = {
  audio: GradioFile;
  metadata: MiniMaxTTSParams & MetadataHeaders;
  folder_root?: string;
};

export function useMiniMaxTTSPage() {
  const [minimaxTTSParams, setMiniMaxTTSParams] =
    useLocalStorage<MiniMaxTTSParams>(minimaxTTSId, defaultMiniMaxTTSParams);

  // Older versions of this page kept the API key in the browser's local
  // storage. The key is held by the server now: drop the copy.
  React.useEffect(() => {
    if ("api_key" in minimaxTTSParams) {
      const { api_key: _dropped, ...rest } = minimaxTTSParams;
      setMiniMaxTTSParams(rest as MiniMaxTTSParams);
    }
  }, [minimaxTTSParams, setMiniMaxTTSParams]);

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
