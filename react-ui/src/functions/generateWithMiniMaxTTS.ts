import { MiniMaxTTSParams, MiniMaxTTSResult } from "../tabs/MiniMaxTTSParams";

/** The Go core calls MiniMax (core/react_ui_backend/go_minimax_tts.go). */
export async function generateWithMiniMaxTTS(
  minimaxTTSParams: MiniMaxTTSParams
) {
  const response = await fetch("/api/go-data/minimax-tts", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(minimaxTTSParams),
  });
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      message = (await response.json()).error ?? message;
    } catch {
      // not JSON
    }
    throw new Error(message);
  }
  return (await response.json()) as MiniMaxTTSResult;
}
