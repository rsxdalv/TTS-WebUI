import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { BarkResult } from "../tabs/BarkResult";

export const generateWithBark = (barkGenerationParams: BarkGenerationParams) =>
  fetch("/api/gradio/bark", {
    method: "POST",
    body: JSON.stringify(barkGenerationParams),
  }).then((response) => response.json() as Promise<BarkResult>);
