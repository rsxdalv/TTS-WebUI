import { updateLocalStorageWithFunction } from "../hooks/useLocalStorage";
import router from "next/router";

export const barkSettingsId = "barkSettingsParams.v4";

export type BarkSettingsParams = {
  text_use_gpu: boolean;
  text_use_small: boolean;
  coarse_use_gpu: boolean;
  coarse_use_small: boolean;
  fine_use_gpu: boolean;
  fine_use_small: boolean;
  codec_use_gpu: boolean;
  load_models_on_startup: boolean;
};

export const initialBarkSettingsParams: BarkSettingsParams = {
  text_use_gpu: false,
  text_use_small: false,
  coarse_use_gpu: false,
  coarse_use_small: false,
  fine_use_gpu: false,
  fine_use_small: false,
  codec_use_gpu: false,
  load_models_on_startup: false,
};

export const sendToBarkSettings = (melody?: string) => {
  if (!melody) return;
  updateLocalStorageWithFunction(
    barkSettingsId,
    (barkSettingsParams: BarkSettingsParams = initialBarkSettingsParams) =>
      ({ ...barkSettingsParams, melody }) as BarkSettingsParams
  );
  router.push("/barkSettings");
};
