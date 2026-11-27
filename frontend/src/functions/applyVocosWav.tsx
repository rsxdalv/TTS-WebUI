import { VocosParams } from "../tabs/VocosParams";
import { GradioFile } from "../types/GradioFile";

export async function applyVocosWav(vocosParams: VocosParams) {
  const response = await fetch("/api/gradio/vocos_wav", {
    method: "POST",
    body: JSON.stringify(vocosParams),
  });

  return (await response.json()) as GradioFile;
}
