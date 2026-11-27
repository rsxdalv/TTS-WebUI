import {
  StableAudioParams,
  StableAudioResult,
} from "../tabs/StableAudioParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithStableAudio(
  stableAudioParams: StableAudioParams
) {
  const body = JSON.stringify(remove_use_random_seed(stableAudioParams));
  const response = await fetch("/api/gradio/stable_audio_generate", {
    method: "POST",
    body,
  });

  return (await response.json()) as StableAudioResult;
}
