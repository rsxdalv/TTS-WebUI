import { Client } from "@gradio/client";
import type { NextApiRequest, NextApiResponse } from "next";
import { getFile } from "../../../backend-utils/getFile";
import { GradioFile } from "../../../types/GradioFile";
import {
  GradioEvent,
  PayloadMessage,
  PredictFunction,
  SubmitIterable,
} from "@gradio/client/dist/types";
import { proxyGradioFilesInResult } from "../../../backend-utils/proxyGradioFile";
import { defaultBackend } from "../../../backend-utils/defaultBackend";

type Data = { data: any };

// import { Client, handle_file } from "@gradio/client";

// const response = await fetch(
// 	"https://audio-samples.github.io/samples/mp3/blizzard_unconditional/sample-0.mp3"
// );
// const audio_file = await response.blob();

// const app = await Client.connect("abidlabs/whisper");
// const result = await app.predict("/predict", [handle_file(audio_file)]);

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse<Data>
) {
  const { name } = req.query;
  // console.log("gradio api handler", name, req.body);

  if (!name || typeof name !== "string" || !endpoints[name]) {
    res.status(404).json({ data: { error: "Not found" } });
    return;
  }

  const { body } = req;
  const parsedBody = body && typeof body === "string" ? JSON.parse(body) : body;
  const result = await endpoints[name](parsedBody);

  res.status(200).json(result);
}

const getClient = () =>
  Client.connect(defaultBackend, {
    auth: process.env.GRADIO_AUTH?.split(":") as [string, string] | undefined,
  });

type GradioChoices = {
  choices: string[];
  __type__: "update";
};

const extractChoices = ({ choices }: GradioChoices) => choices.map((x) => x[0]);
const extractChoicesTuple = ({ choices }: GradioChoices) =>
  choices.map((x) => x[1]);
const getChoices = (result: { data: GradioChoices[] }) =>
  extractChoices(result?.data[0]);

const gradioPredict = <T extends any[]>(...args: Parameters<PredictFunction>) =>
  getClient()
    .then((app) => app.predict(...args) as Promise<{ data: T }>)
    .then<{ data: T }>(proxyGradioFilesInResult);

const gradioSubmit = <T extends any[]>(...args: Parameters<PredictFunction>) =>
  getClient().then(
    (app) =>
      app.submit(...args) as SubmitIterable<
        ({ data: T } & PayloadMessage) | GradioEvent
      >
  );

async function musicgen({ melody, model, ...params }) {
  const melodyBlob = await getFile(melody);

  const result = await gradioPredict<[GradioFile, Object, string]>(
    "/musicgen",
    {
      melody: melodyBlob,
      model_name: model,
      ...params,
    }
  );
  const [audio, metadata, folder_root] = result?.data;
  return { audio, metadata, folder_root };
}

const stable_audio_generate = async ({ init_audio, text, ...params }) => {
  const exampleAudio = await getFile(init_audio);
  const result = await gradioPredict("/stable_audio_generate", {
    prompt: text, // temporary fix until forking stable-audio
    // prompt: "Hello!!",
    // negative_prompt: "Hello!!",
    // seconds_start: 0,
    // seconds_total: 0,
    // cfg_scale: 0,
    // steps: 1,
    // preview_every: 0,
    // seed: "Hello!!",
    // sampler_type: "dpmpp-2m-sde",
    // sigma_min: 0,
    // sigma_max: 0,
    // cfg_rescale: 0,
    // use_init: true,
    init_audio: exampleAudio,
    // init_noise_level: 0.1,
    ...params,
  });

  const [audio, gallery] = result?.data;
  return { audio, gallery };
};

const bark_voice_tokenizer_load = ({ tokenizer, use_gpu }) =>
  gradioPredict<[string]>("/bark_voice_tokenizer_load", [
    tokenizer,
    use_gpu,
  ]).then((result) => result?.data);

async function bark_voice_generate({ audio, use_gpu }) {
  const audioBlob = await getFile(audio);

  const result = await gradioPredict<[string, GradioFile]>(
    "/bark_voice_generate",
    [audioBlob, use_gpu]
  );

  const [filename, preview] = result?.data;
  return { filename, preview };
}

async function bark({
  burn_in_prompt,
  text,
  text_temp,
  waveform_temp,
  long_prompt_radio,
  long_prompt_history_radio,
  seed,
  max_length,

  history_prompt,
  history_prompt_semantic,
}) {
  const result = await gradioPredict<[GradioFile, string, Object, string]>(
    "/bark",
    {
      seed,
      text,
      burn_in_prompt,
      text_temp,
      waveform_temp,
      max_length,
      history_prompt,
      history_prompt_semantic,
      long_prompt_radio,
      long_prompt_history_radio,
    }
  );
  const [audio, npz, metadata, folder_root] = result?.data;
  return {
    audio: { ...audio, data: audio.url },
    npz,
    metadata,
    folder_root,
  };
}

const bark_favorite = async ({ folder_root }) =>
  gradioPredict<[Object]>("/bark_favorite", [folder_root]).then(
    (result) => result?.data
  );

async function Array__fromAsync<T>(gen: AsyncIterable<T>): Promise<T[]> {
  let out: T[] = [];
  for await (const x of gen) {
    // Oh how I love you V8 with your bugs. This prevents invalid memory deduplication when adding the output to an array in a complicated system.
    let y = JSON.parse(JSON.stringify(x));
    out.push(y as any);
  }
  return out;
}

async function tortoise({
  text,
  speaker,
  preset,
  seed,
  cvvp_amount,
  split_prompt,
  samples,
  diffusion_iterations,
  temperature,
  length_penalty,
  repetition_penalty,
  top_p,
  max_mel_tokens,
  cond_free,
  cond_free_k,
  diffusion_temperature,
  model,
  generation_name,
  // use_random_seed,
  candidates,
}) {
  const job = await gradioSubmit<[GradioFile, string, Object]>(
    "/generate_tortoise_" + candidates,
    [
      text, // string  in 'Prompt' Textbox component
      speaker, // string (Option from: ['random', 'angie', 'applejack', 'cond_latent_example', 'daniel', 'deniro', 'emma', 'freeman', 'geralt', 'halle', 'jlaw', 'lj', 'mol', 'myself', 'pat', 'pat2', 'rainbow', 'snakes', 'tim_reynolds', 'tom', 'train_atkins', 'train_daws', 'train_dotrice', 'train_dreams', 'train_empire', 'train_grace', 'train_kennard', 'train_lescault', 'train_mouse', 'weaver', 'william', 'freeman_2a', 'freeman_3', 'pat4']) in 'parameter_2502' Dropdown component
      preset, // string (Option from: ['ultra_fast', 'fast', 'standard', 'high_quality']) in 'parameter_2507' Dropdown component
      seed, // number  in 'parameter_2521' Number component
      cvvp_amount, // number (numeric value between 0.0 and 1.0) in 'CVVP Amount' Slider component
      split_prompt, // boolean  in 'Split prompt by lines' Checkbox component
      samples, // number (numeric value between 4 and 256) in 'Samples' Slider component
      diffusion_iterations, // number (numeric value between 4 and 400) in 'Diffusion Iterations' Slider component
      temperature, // number (numeric value between 0.0 and 1.0) in 'Temperature' Slider component
      length_penalty, // number (numeric value between 0.0 and 10.0) in 'Length Penalty' Slider component
      repetition_penalty, // number (numeric value between 0.0 and 10.0) in 'Repetition Penalty' Slider component
      top_p, // number (numeric value between 0.0 and 1.0) in 'Top P' Slider component
      max_mel_tokens, // number (numeric value between 10 and 600) in 'Max Mel Tokens' Slider component
      cond_free, // boolean  in 'Cond Free' Checkbox component
      cond_free_k, // number (numeric value between 0 and 10) in 'Cond Free K' Slider component
      diffusion_temperature, // number (numeric value between 0.0 and 1.0) in 'Temperature' Slider component
      model, // string (Option from: ['Default']) in 'parameter_2488' Dropdown component
      generation_name, // string  in 'Generation Name' Textbox component
    ]
  );

  const events = await Array__fromAsync(job);
  const results = events
    .filter<PayloadMessage>((x): x is PayloadMessage => x.type === "data")
    .map<any>(proxyGradioFilesInResult)
    .map((x: PayloadMessage) => {
      const [audio, bundle_name, metadata] = x.data;
      return {
        audio,
        seed,
        bundle_name,
        metadata,
      };
    });
  // remove last element due to return {} in generate_tortoise_long
  return results.slice(0, -1);
}

async function tortoise_apply_model_settings({
  model, // string (Option from: ['Default']) in 'parameter_2488' Dropdown component
  kv_cache, // boolean  in 'parameter_2493' Checkbox component
  use_deepspeed, // boolean  in 'parameter_2494' Checkbox component
  half, // boolean  in 'parameter_2495' Checkbox component
  tokenizer, // string (Option from: ['quantifier_hubert_base_ls960.pth @ GitMylo/bark-voice-cloning', 'quantifier_hubert_base_ls960_14.pth @ GitMylo/bark-voice-cloning', 'quantifier_V1_hubert_base_ls960_23.pth @ GitMylo/bark-voice-cloning', 'polish-HuBERT-quantizer_8_epoch.pth @ Hobis/bark-voice-cloning-polish-HuBERT-quantizer', 'german-HuBERT-quantizer_14_epoch.pth @ CountFloyd/bark-voice-cloning-german-HuBERT-quantizer', 'es_tokenizer.pth @ Lancer1408/bark-es-tokenizer', 'portuguese-HuBERT-quantizer_24_epoch.pth @ MadVoyager/bark-voice-cloning-portuguese-HuBERT-quantizer']) in 'Tokenizer' Dropdown component
  use_basic_cleaners, // boolean  in 'parameter_2497' Checkbox component
}) {
  const tokenizer_file = await getFile(tokenizer);

  const result = await gradioPredict<[Object]>(
    "/tortoise_apply_model_settings",
    [
      model, // string (Option from: ['Default']) in 'parameter_2488' Dropdown component
      kv_cache, // boolean  in 'parameter_2493' Checkbox component
      use_deepspeed, // boolean  in 'parameter_2494' Checkbox component
      half, // boolean  in 'parameter_2495' Checkbox component
      // tokenizer, // string (Option from: ['quantifier_hubert_base_ls960.pth @ GitMylo/bark-voice-cloning', 'quantifier_hubert_base_ls960_14.pth @ GitMylo/bark-voice-cloning', 'quantifier_V1_hubert_base_ls960_23.pth @ GitMylo/bark-voice-cloning', 'polish-HuBERT-quantizer_8_epoch.pth @ Hobis/bark-voice-cloning-polish-HuBERT-quantizer', 'german-HuBERT-quantizer_14_epoch.pth @ CountFloyd/bark-voice-cloning-german-HuBERT-quantizer', 'es_tokenizer.pth @ Lancer1408/bark-es-tokenizer', 'portuguese-HuBERT-quantizer_24_epoch.pth @ MadVoyager/bark-voice-cloning-portuguese-HuBERT-quantizer']) in 'Tokenizer' Dropdown component
      tokenizer_file, // blob in 'Tokenizer' File component
      use_basic_cleaners, // boolean  in 'parameter_2497' Checkbox component
    ]
  );

  return result?.data;
}

async function rvc({
  pitch_up_key,
  original_audio,
  index,
  pitch_collection_method,
  model,
  search_feature_ratio,
  // device,
  // use_half_precision_model,
  filter_radius_pitch,
  resample_sample_rate,
  voice_envelope_normalizaiton,
  protect_breath_sounds,
}) {
  const original_audioBlob = await getFile(original_audio);

  const result = await gradioPredict<any>("/rvc", {
    pitch_up_key, // string  in 'Pitch Up key' Textbox component
    original_audio_path: original_audioBlob, // blob in 'Original Audio' Audio component
    index_path: index, // string in 'Index' Dropdown component
    pitch_collection_method, // string (Option from: ['harvest', 'reaper', 'melodia']) in 'Pitch Collection Method' Radio component
    model_path: model, // string in 'Model' Dropdown component
    index_rate: search_feature_ratio, // number (numeric value between 0.0 and 1.0) in 'Search Feature Ratio' Slider component
    filter_radius: filter_radius_pitch, // number (numeric value between 0 and 10) in 'Filter Radius (Pitch)' Slider component
    resample_sr: resample_sample_rate, // number (numeric value between 0 and 48000) in 'Resample Sample-rate (Bug)' Slider component
    rms_mix_rate: voice_envelope_normalizaiton, // number (numeric value between 0.0 and 1.0) in 'Voice Envelope Normalizaiton' Slider component
    protect: protect_breath_sounds, // number (numeric value between 0.0 and 0.5) in 'Protect Breath Sounds' Slider component
  });

  const [audio, metadata] = result?.data;
  return { audio, metadata };
}

async function ace_step_infer({
  audio_duration,
  tags,
  lyrics,
  infer_steps,
  guidance_scale,
  scheduler_type,
  cfg_type,
  granularity_scale,
  manual_seeds,
  guidance_interval,
  guidance_interval_decay,
  min_guidance_scale,
  use_erg_for_tag,
  use_erg_for_lyric,
  use_erg_for_diffusion,
  oss_steps,
  guidance_scale_text,
  guidance_scale_lyric,
  audio2audio_enable,
  ref_audio_strength,
  ref_audio_input,
}) {
  const ref_audio_inputBlob = await getFile(ref_audio_input);

  const result = await gradioPredict<any>("/ace_step_infer", {
    audio_duration, // number (numeric value between 0.0 and 10.0) in 'Audio Duration' Slider component
    prompt: tags, // string  in 'Tags' Textbox component
    lyrics, // string  in 'Lyrics' Textbox component
    infer_step: infer_steps, // number (numeric value between 1 and 1000) in 'Infer Steps' Slider component
    guidance_scale, // number (numeric value between 0.0 and 10.0) in 'Guidance Scale' Slider component
    scheduler_type, // string (Option from: ['euler', 'heun']) in 'Scheduler Type' Radio component
    cfg_type, // string (Option from: ['cfg', 'apg', 'cfg_star']) in 'CFG Type' Radio component
    omega_scale: granularity_scale, // number (numeric value between 0.0 and 10.0) in 'Granularity Scale' Slider component
    manual_seeds, // string  in 'manual seeds (default None)' Textbox component
    guidance_interval, // number (numeric value between 0.0 and 1.0) in 'Guidance Interval' Slider component
    guidance_interval_decay, // number (numeric value between 0.0 and 1.0) in 'Guidance Interval Decay' Slider component
    min_guidance_scale, // number (numeric value between 0.0 and 10.0) in 'Min Guidance Scale' Slider component
    use_erg_tag: use_erg_for_tag, // boolean  in 'use ERG for tag' Checkbox component
    use_erg_lyric: use_erg_for_lyric, // boolean  in 'use ERG for lyric' Checkbox component
    use_erg_diffusion: use_erg_for_diffusion, // boolean  in 'use ERG for diffusion' Checkbox component
    oss_steps, // string  in 'OSS Steps' Textbox component
    guidance_scale_text, // number (numeric value between 0.0 and 10.0) in 'Guidance Scale Text' Slider component
    guidance_scale_lyric, // number (numeric value between 0.0 and 10.0) in 'Guidance Scale Lyric' Slider component
    audio2audio_enable, // boolean  in 'Audio2Audio Enable' Checkbox component
    ref_audio_strength, // number (numeric value between 0.0 and 1.0) in 'Ref Audio Strength' Slider component
    ref_audio_input: ref_audio_inputBlob, // blob in 'Ref Audio Input' Audio component
  });

  const [audio, metadata] = result?.data;
  return { audio, metadata };
}

const delete_generation = ({ folder_root }) =>
  gradioPredict<[]>("/delete_generation", [folder_root]);

async function get_config_bark() {
  const result = await gradioPredict<
    [
      { value: boolean }, // boolean representing output in 'Use GPU' Checkbox component
      { value: boolean }, // boolean representing output in 'Use small model' Checkbox component
      { value: boolean }, // boolean representing output in 'Use GPU' Checkbox component
      { value: boolean }, // boolean representing output in 'Use small model' Checkbox component
      { value: boolean }, // boolean representing output in 'Use GPU' Checkbox component
      { value: boolean }, // boolean representing output in 'Use small model' Checkbox component
      { value: boolean }, // boolean representing output in 'Use GPU for codec' Checkbox component
      { value: boolean }, // boolean representing output in 'Load Bark models on startup' Checkbox component
    ]
  >("/get_config_bark", []);

  const [
    text_use_gpu,
    text_use_small,
    coarse_use_gpu,
    coarse_use_small,
    fine_use_gpu,
    fine_use_small,
    codec_use_gpu,
    load_models_on_startup,
  ] = result?.data.map((x) => x.value);

  return {
    text_use_gpu,
    text_use_small,
    coarse_use_gpu,
    coarse_use_small,
    fine_use_gpu,
    fine_use_small,
    codec_use_gpu,
    load_models_on_startup,
  };
}

const simpleOutputMap = (result: { data: [GradioFile, Object, string] }) => {
  const [audio, metadata, folder_root] = result?.data;
  return { audio, metadata, folder_root };
};

const simpleEndpoint = (endpoint: string) => (params: Record<string, any>) =>
  gradioPredict<[GradioFile, Object, string]>(endpoint, params).then(
    simpleOutputMap
  );

const scan_huggingface_cache_api = () =>
  gradioPredict<any[]>("/scan_huggingface_cache_api").then(
    (result) => result?.data?.[0]
  );

const delete_huggingface_cache_revisions = ({ commit_hash }) =>
  gradioPredict<[]>("/delete_huggingface_cache_revisions", [commit_hash]);

const passThrough = (endpoint: string) => (params: Record<string, any>) =>
  gradioPredict<any>(endpoint, params);

type LocalCacheFile = {
  __type: "local_cache_file";
  path: string;
};

const isLocalCacheFile = (x: any): x is LocalCacheFile =>
  typeof x === "object" && x.__type === "local_cache_file";

const resolveParam = async (_key: string, value: any) =>
  isLocalCacheFile(value) ? await getFile(value.path) : value;

const resolveFileParams = async (params: Record<string, any>) =>
  Object.fromEntries(
    await Promise.all(
      Object.entries(params).map(async ([key, value]) => [
        key,
        await resolveParam(key, value),
      ])
    )
  );

const resolvedPassThrough =
  (endpoint: string, map = (x: any) => x) =>
  (params: Record<string, any>) =>
    resolveFileParams(params)
      .then((resolvedParams) => gradioPredict<any>(endpoint, resolvedParams))
      .then(map);

const resolvedPassThroughWithParamsLoopback =
  (endpoint: string, map = (x: any) => x) =>
  (params: Record<string, any>) =>
    resolveFileParams(params)
      .then((resolvedParams) => gradioPredict<any>(endpoint, resolvedParams))
      .then((result) => ({ ...map(result), metadata: params }));

const endpoints = {
  maha: simpleEndpoint("/maha_tts"),
  maha_tts_refresh_voices: resolvedPassThrough(
    "/maha_tts_refresh_voices",
    getChoices
  ),
  demucs: resolvedPassThrough("/demucs", (result) => result?.data),
  musicgen,
  magnet: simpleEndpoint("/magnet"),
  // magnet_get_models,
  magnet_get_models: resolvedPassThrough("/magnet_get_models", (result) =>
    extractChoicesTuple(result?.data[0])
  ),
  // magnet_open_model_dir,
  magnet_open_model_dir: passThrough("/magnet_open_model_dir"),
  magnet_unload_model: passThrough("/magnet_unload_model"),

  vocos_wav: resolvedPassThrough("/vocos_wav", (result) => result?.data[0]),
  vocos_npz: resolvedPassThrough("/vocos_npz", (result) => result?.data[0]),
  encodec_decode: resolvedPassThrough(
    "/encodec_decode",
    (result) => result?.data[0]
  ),
  bark_voice_tokenizer_load,
  bark_voice_generate,
  bark,
  reload_old_generation_dropdown: resolvedPassThrough(
    "/reload_old_generation_dropdown",
    getChoices
  ),
  bark_favorite,
  delete_generation,

  tortoise,
  tortoise_refresh_models: resolvedPassThrough(
    "/tortoise_refresh_models",
    getChoices
  ),
  tortoise_refresh_voices: resolvedPassThrough(
    "/tortoise_refresh_voices",
    getChoices
  ),
  tortoise_open_models: passThrough("/tortoise_open_models"),
  tortoise_open_voices: passThrough("/tortoise_open_voices"),
  tortoise_apply_model_settings,

  rvc,
  rvc_model_reload: resolvedPassThrough("/rvc_model_reload", getChoices),
  rvc_index_reload: resolvedPassThrough("/rvc_index_reload", getChoices),
  rvc_model_open: passThrough("/rvc_model_open"),
  rvc_index_open: passThrough("/rvc_index_open"),
  rvc_model_unload: passThrough("/rvc_model_unload"),
  save_to_voices: passThrough("/save_to_voices"),
  open_folder: passThrough("/open_folder"),

  save_env_variables_bark: resolvedPassThrough(
    "/save_env_variables_bark",
    (result) => result?.data
  ),
  save_config_bark: resolvedPassThrough(
    "/save_config_bark",
    (result) => result?.data[0]
  ),
  get_config_bark,

  mms: simpleEndpoint("/mms"),

  get_gpu_info: resolvedPassThrough(
    "/get_gpu_info",
    (result) => result?.data[0]
  ),

  vall_e_x_generate: simpleEndpoint("/vall_e_x_generate"),
  vall_e_x_split_text_into_sentences: resolvedPassThrough(
    "/vall_e_x_split_text_into_sentences",
    (result) => ({ split_text: result?.data?.[0] })
  ),
  vall_e_x_tokenize: resolvedPassThrough("/vall_e_x_tokenize", (result) => ({
    tokens: result?.data?.[0],
  })),

  scan_huggingface_cache_api,
  delete_huggingface_cache_revisions,

  // tortoise_unload_model,
  tortoise_unload_model: passThrough("/tortoise_unload_model"),

  musicgen_audiogen_get_models: () =>
    gradioPredict<[GradioChoices]>("/musicgen_audiogen_get_models").then(
      (result) => extractChoicesTuple(result?.data[0])
    ),
  musicgen_audiogen_open_model_dir: passThrough(
    "/musicgen_audiogen_open_model_dir"
  ),
  musicgen_audiogen_unload_model: passThrough(
    "/musicgen_audiogen_unload_model"
  ),
  // stable_audio_generate: passThrough("/stable_audio_generate"),
  stable_audio_generate,
  stable_audio_inpaint: passThrough("/stable_audio_inpaint"),
  stable_audio_get_models: resolvedPassThrough(
    "/stable_audio_get_models",
    (result) => extractChoicesTuple(result?.data[0])
  ),
  load_ffmpeg_metadata: resolvedPassThrough(
    "/load_ffmpeg_metadata",
    (result) => result?.data[0]
  ),

  ace_step_infer,

  kokoro_generate: resolvedPassThroughWithParamsLoopback(
    "/kokoro",
    (result) => ({
      audio: result?.data?.[0],
      tokens: result?.data?.[1],
      metadata: result?.data?.[2],
      folder_root: result?.data?.[3],
    })
  ),

  chatterbox_tts: resolvedPassThroughWithParamsLoopback(
    "/chatterbox_tts",
    (result) => ({
      audio: result?.data?.[0],
      metadata: result?.data?.[1],
      folder_root: result?.data?.[2],
    })
  ),

  //   api/gradio/open_ai_api_load_presets
  // api/gradio/open_ai_api_save_presets
  // api/gradio/get_chatterbox_voices
  // api/gradio/open_ai_api_test_voice_preset
  open_ai_api_load_presets: resolvedPassThrough(
    "/open_ai_api_load_presets",
    (result) => result?.data[0]
  ),
  open_ai_api_save_presets: passThrough("/open_ai_api_save_presets"),
  get_chatterbox_voices: resolvedPassThrough(
    "/get_chatterbox_voices",
    (result) => result?.data[0]
  ),
  open_ai_api_test_voice_preset: resolvedPassThrough(
    "/open_ai_api_test_voice_preset",
    (result) => result?.data[0].url
  ),
};
