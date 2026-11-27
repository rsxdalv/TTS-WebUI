import {
  AceStepParams,
  AceStepResult,
} from "../tabs/AceStepParams";
import { remove_use_random_seed } from "./remove_use_random_seed";

export async function generateWithAceStep(
  aceStepParams: AceStepParams
) {
  const params = remove_use_random_seed(aceStepParams);

  // Map the parameters to match the expected API structure
  const apiParams = {
    audio_duration: params.audio_duration,
    tags: params.tags,
    lyrics: params.lyrics,
    infer_steps: params.infer_steps,
    guidance_scale: params.guidance_scale,
    scheduler_type: params.scheduler_type,
    cfg_type: params.cfg_type,
    granularity_scale: params.granularity_scale,
    manual_seeds: params.manual_seeds || params.seed.toString(),
    guidance_interval: params.guidance_interval,
    guidance_interval_decay: params.guidance_interval_decay,
    min_guidance_scale: params.min_guidance_scale,
    use_erg_for_tag: params.use_erg_for_tag,
    use_erg_for_lyric: params.use_erg_for_lyric,
    use_erg_for_diffusion: params.use_erg_for_diffusion,
    oss_steps: params.oss_steps,
    // check if empty
    guidance_scale_text: params.guidance_scale_text,
    guidance_scale_lyric: params.guidance_scale_lyric,
    audio2audio_enable: params.audio2audio_enable,
    ref_audio_strength: params.ref_audio_strength,
    ref_audio_input: params.ref_audio_input,
  };

  const body = JSON.stringify(apiParams);
  const response = await fetch("/api/gradio/ace_step_infer", {
    method: "POST",
    body,
  });

  return (await response.json()) as AceStepResult;
}
