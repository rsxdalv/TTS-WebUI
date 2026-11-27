import { MahaParams, MahaResult } from "../tabs/MahaParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithMaha(mahaParams: MahaParams) {
  // const body = JSON.stringify({ ...mahaParams });
  const body = JSON.stringify(remove_use_random_seed(mahaParams));
  const response = await fetch("/api/gradio/maha", {
    method: "POST",
    body,
  });

  return (await response.json()) as MahaResult;
}
