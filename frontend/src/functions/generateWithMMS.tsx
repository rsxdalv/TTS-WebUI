import { MMSParams, MMSResult } from "../tabs/MMSParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithMMS(mmsParams: MMSParams) {
  const body = JSON.stringify(remove_use_random_seed(mmsParams));
  const response = await fetch("/api/gradio/mms", {
    method: "POST",
    body,
  });

  return (await response.json()) as MMSResult;
}
