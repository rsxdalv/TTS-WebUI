import { MiniMaxTTSParams, MiniMaxTTSResult } from "../tabs/MiniMaxTTSParams";

export async function generateWithMiniMaxTTS(
  minimaxTTSParams: MiniMaxTTSParams
) {
  const body = JSON.stringify(minimaxTTSParams);
  const response = await fetch("/api/gradio/minimax_cloud_tts", {
    method: "POST",
    body,
  });

  return (await response.json()) as MiniMaxTTSResult;
}
