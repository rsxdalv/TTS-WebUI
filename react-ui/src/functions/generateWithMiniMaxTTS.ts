import { MiniMaxTTSParams, MiniMaxTTSResult } from "../tabs/MiniMaxTTSParams";
import { getWebuiURL } from "../data/getWebuiURL";
import { goGetJson, goSendJson } from "./goData";

/** What the Go core answers to a generation (core/internal/cloud/minimax). */
export type MiniMaxClip = {
  folder: string;
  wav: string;
  png: string;
  json: string;
  seconds: number;
  text: string;
  model: string;
  voice_id: string;
};

/** The settings as the Go core lists them: the key is never in it. */
export type MiniMaxSettings = {
  base_url: string;
  model: string;
  voice: string;
  has_key: boolean;
  /** "stored" (saved here), "env" (MINIMAX_API_KEY) or "none". */
  key_source: "stored" | "env" | "none";
};

export const MINIMAX_SETTINGS = "/api/go-data/minimax/settings";

export const getMiniMaxSettings = () =>
  goGetJson<MiniMaxSettings>(MINIMAX_SETTINGS);

/**
 * Generates with MiniMax through the Go core (no Gradio host). The key is not
 * sent from here: Go holds it. The clip is saved in outputs/ by Go.
 */
export async function generateWithMiniMaxTTS(
  minimaxTTSParams: MiniMaxTTSParams
): Promise<MiniMaxTTSResult> {
  const clip = await goSendJson<MiniMaxClip>(
    "POST",
    "/api/go-data/minimax/speech",
    {
      text: minimaxTTSParams.text,
      model: minimaxTTSParams.model,
      voice_id: minimaxTTSParams.voice_id,
    }
  );
  const url = getWebuiURL(clip.wav);
  // A key typed on an older version of the page must not end up in the history.
  const { api_key: _key, ...params } = minimaxTTSParams;
  return {
    audio: {
      name: clip.wav,
      data: "",
      url,
      is_file: true,
      orig_name: clip.wav.split("/").pop(),
    },
    metadata: {
      ...params,
      text: clip.text,
      model: clip.model,
      voice_id: clip.voice_id,
      provider: "minimax",
      _type: "minimax_cloud_tts",
      _version: "0.1.0",
      _hash_version: "0.0.2",
    },
    folder_root: clip.folder,
  };
}
