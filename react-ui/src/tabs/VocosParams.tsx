import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { LocalCacheFile, toLocalCacheFile } from "../types/LocalCacheFile";

export type VocosParams = {
  audio?: LocalCacheFile;
  bandwidth?: string;
};

export const initialState: VocosParams = {
  audio: toLocalCacheFile(
    "https://www.mfiles.co.uk/mp3-downloads/gs-cd-track2.mp3"
  ),
  bandwidth: "1.5",
};

export type VocosResult = GradioFile;

export const vocosId = "vocos_wav-tab";

export const sendToVocos = (audio?: string) => {
  if (!audio) return;
  updateLocalStorageWithFunction(
    vocosId,
    (vocosParams: VocosParams = initialState) => ({
      ...vocosParams,
      audio: toLocalCacheFile(audio),
    })
  );
  router.push("/vocos_wav");
};

export const useVocosParams = () =>
  useLocalStorage<VocosParams>(vocosId, initialState);

export const useVocosResults = () =>
  useLocalStorage<VocosResult | null>(vocosId + ".output", null);

export const getVocosParams = (): VocosParams =>
  readLocalStorage(vocosId) ?? initialState;
