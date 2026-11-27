import { DemucsParams } from "../tabs/DemucsParams";

export async function splitWithDemucs(demucsParams: DemucsParams) {
  const response = await fetch("/api/gradio/demucs", {
    method: "POST",
    body: JSON.stringify(demucsParams),
  });

  return await response.json();
}
