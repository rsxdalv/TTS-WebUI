import useLocalStorage, {
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { LocalCacheFile, toLocalCacheFile } from "../types/LocalCacheFile";

export type EncodecParamsNPZ = {
  npz_file?: LocalCacheFile;
};

export const initialState: EncodecParamsNPZ = {
  npz_file: toLocalCacheFile(
    "https://www.mfiles.co.uk/mp3-downloads/gs-cd-track2.mp3"
  ),
};

export type VocosResultNPZ = GradioFile;

export const vocosIdNPZ = "vocos_npz-tab";

export const sendToVocosNPZ = (npz_file?: string) => {
  if (!npz_file) return;
  updateLocalStorageWithFunction(
    vocosIdNPZ,
    (vocosParams: EncodecParamsNPZ = initialState) => ({
      ...vocosParams,
      npz_file: toLocalCacheFile(npz_file),
    })
  );
  router.push("/vocos_npz");
};

export const useVocosParamsNPZ = () =>
  useLocalStorage<EncodecParamsNPZ>(vocosIdNPZ, initialState);

export const useVocosResultsNPZ = () =>
  useLocalStorage<VocosResultNPZ | null>(vocosIdNPZ + ".output", null);
