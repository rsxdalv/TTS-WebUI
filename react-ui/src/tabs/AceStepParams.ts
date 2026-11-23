import useLocalStorage from "../hooks/useLocalStorage";
import { useHistory } from "../hooks/useHistory";
import { parseFormChange } from "../data/parseFormChange";
import { useSeedHelper } from "../functions/results/useSeedHelper";
import { favorite } from "../functions/favorite";
import { Seeded } from "../types/Seeded";
import { GradioFile } from "../types/GradioFile";
import router from "@/shims/router";

export const aceStepId = "aceStepParams.v1";

export const sendToAceStep = (audio?: string) => {
  if (!audio) return;
  // For ACE-Step, we don't have a direct audio input parameter
  // Just navigate to the page
  router.push("/audio-music-generation/ace-step");
};

export type AceStepParams = Seeded & {
  audio_duration: number;
  tags: string;
  lyrics: string;
  infer_steps: number;
  guidance_scale: number;
  scheduler_type: string;
  cfg_type: string;
  granularity_scale: number;
  guidance_interval: number;
  guidance_interval_decay: number;
  min_guidance_scale: number;
  use_erg_for_tag: boolean;
  use_erg_for_lyric: boolean;
  use_erg_for_diffusion: boolean;
  oss_steps: string;
  guidance_scale_text: number;
  guidance_scale_lyric: number;
  manual_seeds?: string;
  audio2audio_enable: boolean;
  ref_audio_strength: number;
  ref_audio_input?: string;
};

// export const initialAceStepParams: AceStepParams = {
//   seed: 0,
//   use_random_seed: true,

//   audio_duration: 6.0,
//   tags: "lofi hip hop beats to relax/study to",
//   lyrics: "",
//   infer_steps: 100,
//   guidance_scale: 7.0,
//   scheduler_type: "euler",
//   cfg_type: "cfg",
//   granularity_scale: 1.0,
//   guidance_interval: 0.5,
//   guidance_interval_decay: 0.5,
//   min_guidance_scale: 1.0,
//   use_erg_for_tag: true,
//   use_erg_for_lyric: true,
//   use_erg_for_diffusion: true,
//   oss_steps: "",
//   guidance_scale_text: 7.0,
//   guidance_scale_lyric: 7.0,
// };

// param_0 number Default: -1

// The input value that is provided in the "Audio Duration" Slider component.

// param_1 string Default: "funk, pop, soul, rock, melodic, guitar, drums, bass, keyboard, percussion, 105 BPM, energetic, upbeat, groovy, vibrant, dynamic"

// The input value that is provided in the "Tags" Textbox component.

// param_2 string Default: "[verse] Neon lights they flicker bright City hums in dead of night Rhythms pulse through concrete veins Lost in echoes of refrains [verse] Bassline groovin' in my chest Heartbeats match the city's zest Electric whispers fill the air Synthesized dreams everywhere [chorus] Turn it up and let it flow Feel the fire let it grow In this rhythm we belong Hear the night sing out our song [verse] Guitar strings they start to weep Wake the soul from silent sleep Every note a story told In this night we’re bold and gold [bridge] Voices blend in harmony Lost in pure cacophony Timeless echoes timeless cries Soulful shouts beneath the skies [verse] Keyboard dances on the keys Melodies on evening breeze Catch the tune and hold it tight In this moment we take flight "

// The input value that is provided in the "Lyrics" Textbox component.

// param_3 number Default: 27

// The input value that is provided in the "Infer Steps" Slider component.

// param_4 number Default: 15

// The input value that is provided in the "Guidance Scale" Slider component.

// param_5 string Default: "euler"

// The input value that is provided in the "Scheduler Type" Radio component.

// param_6 string Default: "apg"

// The input value that is provided in the "CFG Type" Radio component.

// param_7 number Default: 10

// The input value that is provided in the "Granularity Scale" Slider component.

// param_8 string Required

// The input value that is provided in the "manual seeds (default None)" Textbox component.

// param_9 number Default: 0.5

// The input value that is provided in the "Guidance Interval" Slider component.

// param_10 number Default: 0

// The input value that is provided in the "Guidance Interval Decay" Slider component.

// param_11 number Default: 3

// The input value that is provided in the "Min Guidance Scale" Slider component.

// param_12 boolean Default: True

// The input value that is provided in the "use ERG for tag" Checkbox component.

// param_13 boolean Default: True

// The input value that is provided in the "use ERG for lyric" Checkbox component.

// param_14 boolean Default: True

// The input value that is provided in the "use ERG for diffusion" Checkbox component.

// param_15 string Required

// The input value that is provided in the "OSS Steps" Textbox component.

// param_16 number Default: 0

// The input value that is provided in the "Guidance Scale Text" Slider component.

// param_17 number Default: 0

// The input value that is provided in the "Guidance Scale Lyric" Slider component.

export const initialAceStepParams: AceStepParams = {
  seed: 0,
  use_random_seed: true,

  audio_duration: -1,
  tags: "funk, pop, soul, rock, melodic, guitar, drums, bass, keyboard, percussion, 105 BPM, energetic, upbeat, groovy, vibrant, dynamic",
  lyrics: `[verse]
Neon lights they flicker bright
City hums in dead of night
Rhythms pulse through concrete veins
Lost in echoes of refrains

[verse]
Bassline groovin' in my chest
Heartbeats match the city's zest
Electric whispers fill the air
Synthesized dreams everywhere

[chorus]
Turn it up and let it flow
Feel the fire let it grow
In this rhythm we belong
Hear the night sing out our song

[verse]
Guitar strings they start to weep
Wake the soul from silent sleep
Every note a story told
In this night we’re bold and gold

[bridge]
Voices blend in harmony
Lost in pure cacophony
Timeless echoes timeless cries
Soulful shouts beneath the skies

[verse]
Keyboard dances on the keys
Melodies on evening breeze
Catch the tune and hold it tight
In this moment we take flight
`,
  infer_steps: 27,
  guidance_scale: 15,
  scheduler_type: "euler",
  cfg_type: "apg",
  granularity_scale: 10,
  guidance_interval: 0.5,
  guidance_interval_decay: 0,
  min_guidance_scale: 3,
  use_erg_for_tag: true,
  use_erg_for_lyric: true,
  use_erg_for_diffusion: true,
  oss_steps: "",
  guidance_scale_text: 0,
  guidance_scale_lyric: 0,
  manual_seeds: "",
  audio2audio_enable: false,
  ref_audio_strength: 0.5,
  ref_audio_input: undefined,
};

export type AceStepResult = {
  audio: GradioFile;
  metadata: {
    audio_duration: number;
    tags: string;
    lyrics: string;
    infer_steps: number;
    guidance_scale: number;
    scheduler_type: string;
    cfg_type: string;
    granularity_scale: number;
    guidance_interval: number;
    guidance_interval_decay: number;
    min_guidance_scale: number;
    use_erg_for_tag: boolean;
    use_erg_for_lyric: boolean;
    use_erg_for_diffusion: boolean;
    oss_steps: string;
    guidance_scale_text: number;
    guidance_scale_lyric: number;
    manual_seeds?: string;
    date: string;
    seed: string;
  };
};

export function useAceStepPage() {
  const [aceStepParams, setAceStepParams] = useLocalStorage(
    aceStepId,
    initialAceStepParams
  );

  const [historyData, setHistoryData] = useHistory<AceStepResult>("aceStep");

  const consumer = async (params: AceStepParams) => {
    const result = await generateWithAceStep(params);
    setHistoryData((prev) => [result, ...prev]);
    return result;
  };

  const funcs = {
    favorite: (metadata: any) => favorite(metadata),
    useSeed: useSeedHelper(setAceStepParams),
    useParameters: (_url: string, data?: AceStepResult) => {
      const params = data?.metadata;
      if (!params) return;
      setAceStepParams({
        ...aceStepParams,
        ...params,
        seed: Number(params.seed),
      });
    },
  };

  return {
    aceStepParams,
    setAceStepParams,
    historyData,
    setHistoryData,
    consumer,
    handleChange: parseFormChange(setAceStepParams),
    funcs,
  };
}

// This function will be implemented in a separate file
import { generateWithAceStep } from "../functions/generateWithAceStep";


