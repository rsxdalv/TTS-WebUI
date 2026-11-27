export interface NPZOptional {
  _version: string;
  _hash_version: string;
  _type: string;
  is_big_semantic_model: boolean;
  is_big_coarse_model: boolean;
  is_big_fine_model: boolean;
  prompt: string;
  language: any;
  speaker_id: any;
  hash: string;
  history_prompt: string;
  history_prompt_npz: any;
  history_hash: string;
  text_temp: number;
  waveform_temp: number;
  date: string;
  seed: string;
  crop_min: number;
  crop_max: number;
}

export interface NPZ extends Partial<NPZOptional> {
  filename: string;
  url: string;
}
