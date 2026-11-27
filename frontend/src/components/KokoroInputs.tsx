import React, { useState } from "react";
import { KokoroParams, initialKokoroParams } from "../tabs/KokoroParams";
import { PromptTextArea } from "./PromptTextArea";
import { HandleChange } from "../types/HandleChange";
import { ParameterSlider } from "./GenericSlider";
import { Label } from "./ui/label";
import { Switch } from "./ui/switch";
import { ResetButton } from "./ResetButton";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select";
import { SeedInput } from "./SeedInput";
import { Input } from "./ui/input";
import { Separator } from "react-aria-components";

interface KokoroInputsProps {
  kokoroParams: KokoroParams;
  handleChange: HandleChange;
  setKokoroParams: React.Dispatch<React.SetStateAction<KokoroParams>>;
  resetParams: () => void;
}

const CHOICES = {
  "🇺🇸 🚺 Heart ❤️": "af_heart",
  "🇺🇸 🚺 Bella 🔥": "af_bella",
  "🇺🇸 🚺 Nicole 🎧": "af_nicole",
  "🇺🇸 🚺 Aoede": "af_aoede",
  "🇺🇸 🚺 Kore": "af_kore",
  "🇺🇸 🚺 Sarah": "af_sarah",
  "🇺🇸 🚺 Nova": "af_nova",
  "🇺🇸 🚺 Sky": "af_sky",
  "🇺🇸 🚺 Alloy": "af_alloy",
  "🇺🇸 🚺 Jessica": "af_jessica",
  "🇺🇸 🚺 River": "af_river",
  "🇺🇸 🚹 Michael": "am_michael",
  "🇺🇸 🚹 Fenrir": "am_fenrir",
  "🇺🇸 🚹 Puck": "am_puck",
  "🇺🇸 🚹 Echo": "am_echo",
  "🇺🇸 🚹 Eric": "am_eric",
  "🇺🇸 🚹 Liam": "am_liam",
  "🇺🇸 🚹 Onyx": "am_onyx",
  "🇺🇸 🚹 Santa": "am_santa",
  "🇺🇸 🚹 Adam": "am_adam",
  "🇬🇧 🚺 Emma": "bf_emma",
  "🇬🇧 🚺 Isabella": "bf_isabella",
  "🇬🇧 🚺 Alice": "bf_alice",
  "🇬🇧 🚺 Lily": "bf_lily",
  "🇬🇧 🚹 George": "bm_george",
  "🇬🇧 🚹 Fable": "bm_fable",
  // # "🇬🇧 🚹 Lewis": "bm_lewis", # Suspicious data.
  "🇬🇧 🚹 Daniel": "bm_daniel",
  "🇪🇸 🚺 Dora": "ef_dora",
  "🇪🇸 🚹 Alex": "em_alex",
  "🇪🇸 🚹 Santa": "em_santa",
  "🇫🇷 🚺 Siwis": "ff_siwis",
  "🇮🇳 🚺 Alpha": "hf_alpha",
  "🇮🇳 🚺 Beta": "hf_beta",
  "🇮🇳 🚹 Omega": "hm_omega",
  "🇮🇳 🚹 Psi": "hm_psi",
  "🇮🇹 🚺 Sara": "if_sara",
  "🇮🇹 🚹 Nicola": "im_nicola",
  "🇧🇷 🚺 Dora": "pf_dora",
  "🇧🇷 🚹 Alex": "pm_alex",
  "🇧🇷 🚹 Santa": "pm_santa",
  // # requires fugashi
  "🇯🇵 🚺 Alpha": "jf_alpha",
  "🇯🇵 🚺 Gongitsune": "jf_gongitsune",
  "🇯🇵 🚺 Nezumi": "jf_nezumi",
  "🇯🇵 🚺 Tebukuro": "jf_tebukuro",
  "🇯🇵 🚹 Kumo": "jm_kumo",
  // # requires misaki
  "🇨🇳 🚺 Xiaobei": "zf_xiaobei",
  "🇨🇳 🚺 Xiaoni": "zf_xiaoni",
  "🇨🇳 🚺 Xiaoxiao": "zf_xiaoxiao",
  "🇨🇳 🚺 Xiaoyi": "zf_xiaoyi",
  "🇨🇳 🚹 Yunjian": "zm_yunjian",
  "🇨🇳 🚹 Yunxi": "zm_yunxi",
  "🇨🇳 🚹 Yunxia": "zm_yunxia",
  "🇨🇳 🚹 Yunyang": "zm_yunyang",
};

// Available voices based on Kokoro documentation
// const KOKORO_VOICES = [
//   { value: "af_heart", label: "AF Heart" },
//   { value: "af_nicole", label: "AF Nicole" },
//   { value: "af_sarah", label: "AF Sarah" },
//   { value: "af_sky", label: "AF Sky" },
//   { value: "af_bella", label: "AF Bella" },
//   { value: "af_grace", label: "AF Grace" },
//   { value: "af_emma", label: "AF Emma" },
//   { value: "af_alloy", label: "AF Alloy" },
//   { value: "am_adam", label: "AM Adam" },
//   { value: "am_michael", label: "AM Michael" },
// ];
const KOKORO_VOICES = Object.entries(CHOICES).map(([label, value]) => ({
  label,
  value,
}));

export const KokoroInputs: React.FC<KokoroInputsProps> = ({
  kokoroParams,
  handleChange,
  setKokoroParams,
  resetParams,
}) => {
  const handleCheckboxChange = (name: string) => (checked: boolean) => {
    setKokoroParams((prev) => ({
      ...prev,
      [name]: checked,
    }));
  };

  const handleVoiceChange = (value: string) => {
    setKokoroParams((prev) => ({
      ...prev,
      voice: value,
    }));
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Text Input</CardTitle>
            <CardDescription>
              Enter the text you want to convert to speech using Kokoro TTS.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <PromptTextArea
              params={kokoroParams}
              handleChange={handleChange}
              label=""
              name="text"
              rows={8}
              placeholder="Enter text to synthesize..."
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Voice Selection</CardTitle>
            <CardDescription>
              Choose from available Kokoro voices.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <Label>Pre-defined Voice</Label>
              <Select
                value={kokoroParams.voice}
                onValueChange={handleVoiceChange}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a voice" />
                </SelectTrigger>
                <SelectContent>
                  {KOKORO_VOICES.map((voice) => (
                    <SelectItem key={voice.value} value={voice.value}>
                      {voice.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {/* or a text input for custom voice */}

            <div className="flex items-center space-x-2">
              <Separator className="my-4 flex-1" />
              <Label>or</Label>
              <Separator className="my-4 flex-1" />
            </div>

            <div className="space-y-2">
              <Label>Custom Voice</Label>
              <Input
                type="text"
                name="voice"
                value={kokoroParams.voice}
                onChange={handleChange}
                placeholder="Enter custom voice name"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Speech Parameters</CardTitle>
            <CardDescription>
              Adjust speech speed and other parameters.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <ParameterSlider
              params={kokoroParams}
              onChange={handleChange}
              label="Speed"
              name="speed"
              min="0.25"
              max="4.0"
              step="0.05"
              decimals={2}
            />

            <div className="flex items-center space-x-2">
              <Switch
                id="use_gpu"
                checked={kokoroParams.use_gpu}
                onCheckedChange={handleCheckboxChange("use_gpu")}
              />
              <div>
                <Label htmlFor="use_gpu">Use GPU</Label>
                <CardDescription>
                  Enable GPU acceleration for faster generation.
                </CardDescription>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Seed Settings</CardTitle>
          </CardHeader>
          <CardContent>
            <SeedInput className="flex-wrap" params={kokoroParams} handleChange={handleChange} />
          </CardContent>
        </Card>

        <ResetButton
          params={kokoroParams}
          setParams={setKokoroParams}
          initialParams={initialKokoroParams}
        />
      </div>
    </div>
  );
};
