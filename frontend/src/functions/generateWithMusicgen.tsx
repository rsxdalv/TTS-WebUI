import { MusicgenParams, MusicgenResult } from "../tabs/MusicgenParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithMusicgen(musicgenParams: MusicgenParams) {
  const body = JSON.stringify(remove_use_random_seed(musicgenParams));
  const response = await fetch("/api/gradio/musicgen", {
    method: "POST",
    body,
  });

  return (await response.json()) as MusicgenResult;
}
