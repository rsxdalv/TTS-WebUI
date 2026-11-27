import { GradioFile } from "../types/GradioFile";

export type RVCResult = {
  audio: GradioFile;
  metadata: {
    original_audio_path: string;
    index_path: string;
    model_path: string;
    f0method: string;
    f0up_key: string;
    index_rate: number;
    device: string;
    is_half: boolean;
    filter_radius: number;
    resample_sr: number;
    rms_mix_rate: number;
    protect: number;
  };
};
