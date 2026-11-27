import { updateLocalStorageWithFunction } from "../hooks/useLocalStorage";
import router from "@/shims/router";

export type BarkVoiceGenerationParams = {
  audio?: string;
  tokenizer?: string;
  use_gpu?: boolean;
};

export const initialState: BarkVoiceGenerationParams = {
  audio: "https://www.mfiles.co.uk/mp3-downloads/gs-cd-track2.mp3",
  tokenizer: "quantifier_hubert_base_ls960.pth @ GitMylo/bark-voice-cloning",
  use_gpu: true,
};

export const barkVoiceGenerationId = "bark_voice_generation-tab";

export const sendToBarkVoiceGeneration = (audio?: string) => {
  if (!audio) return;
  updateLocalStorageWithFunction(
    barkVoiceGenerationId,
    (vocosParams: BarkVoiceGenerationParams = initialState) =>
      ({ ...vocosParams, audio } as BarkVoiceGenerationParams)
  );
  router.push("/bark/bark_voice_generation");
};
