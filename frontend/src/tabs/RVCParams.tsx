import { useHistory } from "../hooks/useHistory";
import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { RVCResult } from "./RVCResult";
import { applyRVC } from "../functions/applyRVC";
import { parseFormChange } from "../data/parseFormChange";

export type RVCParams = {
  pitch_up_key: string; // string  in 'Pitch Up key' Textbox component
  original_audio: string; // string  in 'Original Audio' Audio component
  index: string; // string  in 'Index' File component
  pitch_collection_method: string; // string (Option from: ['harvest', 'reaper', 'melodia']) in 'Pitch Collection Method' Radio component
  model: string; // string  in 'Model' File component
  search_feature_ratio: number; // number (numeric value between 0.0 and 1.0) in 'Search Feature Ratio' Slider component
  device: string; // string (Option from: ['cuda:0', 'cpu', 'mps']) in 'Device' Dropdown component
  use_half_precision_model: boolean; // boolean  in 'Use half precision model (Depends on GPU support)' Checkbox component
  filter_radius_pitch: number; // number (numeric value between 0 and 10) in 'Filter Radius (Pitch)' Slider component
  resample_sample_rate: number; // number (numeric value between 0 and 48000) in 'Resample Sample-rate (Bug)' Slider component
  voice_envelope_normalizaiton: number; // number (numeric value between 0.0 and 1.0) in 'Voice Envelope Normalizaiton' Slider component
  protect_breath_sounds: number; // number (numeric value between 0.0 and 0.5) in 'Protect Breath Sounds' Slider component
};

export const initialState: RVCParams = {
  pitch_up_key: "0", // string  in 'Pitch Up key' Textbox component
  original_audio: "", // string  in 'Original Audio' Audio component
  index: "", // string  in 'Index' File component
  pitch_collection_method: "harvest", // string (Option from: ['harvest', 'reaper', 'melodia']) in 'Pitch Collection Method' Radio component
  model: "", // string  in 'Model' File component
  search_feature_ratio: 0.66, // number (numeric value between 0.0 and 1.0) in 'Search Feature Ratio' Slider component
  device: "cuda:0", // string (Option from: ['cuda:0', 'cpu', 'mps']) in 'Device' Dropdown component
  use_half_precision_model: false, // boolean  in 'Use half precision model (Depends on GPU support)' Checkbox component
  filter_radius_pitch: 3, // number (numeric value between 0 and 10) in 'Filter Radius (Pitch)' Slider component
  resample_sample_rate: 0, // number (numeric value between 0 and 48000) in 'Resample Sample-rate (Bug)' Slider component
  voice_envelope_normalizaiton: 0.5, // number (numeric value between 0.0 and 1.0) in 'Voice Envelope Normalizaiton' Slider component
  protect_breath_sounds: 0.33, // number (numeric value between 0.0 and 0.5) in 'Protect Breath Sounds' Slider component
};

export const RVCId = "rvc_generation-tab";

export const sendToRVCGeneration = (audio?: string) => {
  if (!audio) return;
  updateLocalStorageWithFunction(
    RVCId,
    (vocosParams: RVCParams = initialState) =>
      ({ ...vocosParams, audio } as RVCParams)
  );
  router.push("/audio-conversion/rvc");
};

export const useRVCGenerationParams = () =>
  useLocalStorage<RVCParams>(RVCId, initialState);

export const getRVCGenerationParams = (): RVCParams =>
  readLocalStorage(RVCId) ?? initialState;

export const useRVCPage = () => {
  const [rvcGenerationParams, setRvcGenerationParams] =
    useRVCGenerationParams();

  const [historyData, setHistoryData] = useHistory<RVCResult>("rvc");

  const consumer = async (params: RVCParams) => {
    const data = await applyRVC(params);
    setHistoryData((x) => [data, ...x]);
    return data;
  };

  const funcs = {
    useParameters: (_url: string, data?: RVCResult) => {
      const {
        f0up_key: pitch_up_key,
        index_path: index,
        f0method: pitch_collection_method,
        model_path: model,
        index_rate: search_feature_ratio,
        device,
        is_half: use_half_precision_model,
        filter_radius: filter_radius_pitch,
        resample_sr: resample_sample_rate,
        rms_mix_rate: voice_envelope_normalizaiton,
        protect: protect_breath_sounds,
      } = data?.metadata ?? {};

      setRvcGenerationParams({
        ...rvcGenerationParams,
        pitch_up_key: pitch_up_key ?? rvcGenerationParams.pitch_up_key,
        // original_audio,
        index: index ? index.replace(".index", "") : rvcGenerationParams.index,
        pitch_collection_method:
          pitch_collection_method ??
          rvcGenerationParams.pitch_collection_method,
        model: model ? model.replace(".pth", "") : rvcGenerationParams.model,
        search_feature_ratio:
          search_feature_ratio ?? rvcGenerationParams.search_feature_ratio,
        device: device ?? rvcGenerationParams.device,
        use_half_precision_model:
          use_half_precision_model ??
          rvcGenerationParams.use_half_precision_model,
        filter_radius_pitch:
          filter_radius_pitch ?? rvcGenerationParams.filter_radius_pitch,
        resample_sample_rate:
          resample_sample_rate ?? rvcGenerationParams.resample_sample_rate,
        voice_envelope_normalizaiton:
          voice_envelope_normalizaiton ??
          rvcGenerationParams.voice_envelope_normalizaiton,
        protect_breath_sounds:
          protect_breath_sounds ?? rvcGenerationParams.protect_breath_sounds,
      });
    },
  };

  return {
    rvcGenerationParams,
    setRvcGenerationParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setRvcGenerationParams),
    funcs,
  };
};
