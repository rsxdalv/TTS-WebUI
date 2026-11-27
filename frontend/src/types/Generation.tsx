export interface GenerationRaw {
  _type?: string;

  prompt?: string;
  text?: string;
  language: string;
  speaker_id: string;
  history_prompt: string;
  history_prompt_npz: string;
  history_hash: string;
  text_temp: number;
  waveform_temp: number;
  date: string;
  seed: string;
  filename: string;
  filename_png: string;
  filename_json: string;
  filename_npz: string;

  is_big_semantic_model: boolean;
  is_big_coarse_model: boolean;
  is_big_fine_model: boolean;

  hash?: string;
  name?: String;
  folder_root?: string;
  api_filename?: string;
}

export interface TortoiseMeta {
  _version: string;
  _type: string;
  date: string;
  candidates: number;
  text: string;
  voice: string;
  preset: string;
  seed: string;
  cvvp_amount: number;
  split_prompt: boolean;
  num_autoregressive_samples: number;
  diffusion_iterations: number;
  temperature: number;
  length_penalty: number;
  repetition_penalty: number;
  top_p: number;
  max_mel_tokens: number;
  cond_free: boolean;
  cond_free_k: number;
  diffusion_temperature: number;
  model: string;
  name: string;
}
