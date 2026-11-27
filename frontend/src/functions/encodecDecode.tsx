import { EncodecParamsNPZ } from "../tabs/VocosParamsNPZ";

export async function encodecDecode(vocosParams: EncodecParamsNPZ) {
  const response = await fetch("/api/gradio/encodec_decode", {
    method: "POST",
    body: JSON.stringify(vocosParams),
  });

  return await response.json();
}
