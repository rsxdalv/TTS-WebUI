import {
  EncodecParamsNPZ,
  VocosResultNPZ
} from "../tabs/VocosParamsNPZ";

export async function applyVocosNPZ(vocosParams: EncodecParamsNPZ) {
  const response = await fetch("/api/gradio/vocos_npz", {
    method: "POST",
    body: JSON.stringify(vocosParams),
  });

  return (await response.json()) as VocosResultNPZ;
}
