import {
  KokoroParams,
  KokoroResult
} from "../tabs/KokoroParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithKokoro(kokoroParams: KokoroParams) {
  const body = JSON.stringify(remove_use_random_seed(kokoroParams));
  const response = await fetch("/api/gradio/kokoro_generate", {
    method: "POST",
    body,
  });

  return (await response.json()) as KokoroResult;
}
