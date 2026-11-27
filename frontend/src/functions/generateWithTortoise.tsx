import { TortoiseGenerationParams } from "../tabs/TortoiseGenerationParams";
import { TortoiseResult } from "../tabs/TortoiseResult";

export async function generateWithTortoise(
  tortoiseGenerationParams: TortoiseGenerationParams
) {
  const response = await fetch("/api/gradio/tortoise", {
    method: "POST",
    body: JSON.stringify(tortoiseGenerationParams),
  });

  return response.json() as Promise<TortoiseResult[]>;
}
