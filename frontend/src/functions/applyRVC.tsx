import { RVCParams } from "../tabs/RVCParams";
import { RVCResult } from "../tabs/RVCResult";

export async function applyRVC(rvcGenerationParams: RVCParams) {
  const response = await fetch("/api/gradio/rvc", {
    method: "POST",
    body: JSON.stringify(rvcGenerationParams),
  });

  return (await response.json()) as RVCResult;
}
