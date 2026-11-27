import { MagnetParams, MagnetResult } from "../tabs/MagnetParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithMagnet(magnetParams: MagnetParams) {
  // const body = JSON.stringify({ ...magnetParams });
  const body = JSON.stringify(remove_use_random_seed(magnetParams));
  const response = await fetch("/api/gradio/magnet", {
    method: "POST",
    body,
  });

  return (await response.json()) as MagnetResult;
}
