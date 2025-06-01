import {
  ChatterboxParams,
  ChatterboxResult
} from "../tabs/ChatterboxParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithChatterbox(chatterboxParams: ChatterboxParams) {
  const body = JSON.stringify(remove_use_random_seed(chatterboxParams));
  const response = await fetch("/api/gradio/chatterbox_tts", {
    method: "POST",
    body,
  });

  return (await response.json()) as ChatterboxResult;
}
