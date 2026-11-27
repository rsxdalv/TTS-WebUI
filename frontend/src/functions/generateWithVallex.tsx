import { VallexParams, VallexResult } from "../tabs/VallexParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithVallex(vallexParams: VallexParams) {
  // const body = JSON.stringify({ ...vallexParams });
  const body = JSON.stringify(remove_use_random_seed(vallexParams));
  const response = await fetch("/api/gradio/vall_e_x_generate", {
    method: "POST",
    body,
  });

  return response.json() as Promise<VallexResult>;
}
