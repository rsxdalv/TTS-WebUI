import React from "react";
import { HandleChange } from "../types/HandleChange";
import { ChatterboxParams } from "../tabs/ChatterboxParams";
import { PromptTextArea } from "./PromptTextArea";
import { ParameterSlider } from "./GenericSlider";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { toLocalCacheFile } from "../types/LocalCacheFile";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import { UnloadModelButton } from "./component/ModelDropdown";
import { AudioInput } from "./AudioComponents";
import { SeedInput } from "./SeedInput";
import { Switch } from "./ui/switch";
import { Label } from "./ui/label";
import { SelectWithLabel } from "./component/SelectWithLabel";
import {
  BotIcon,
  BrainCogIcon,
  CableIcon,
  CpuIcon,
  KeyboardMusicIcon,
  MemoryStickIcon,
  MicrochipIcon,
  PackageCheckIcon,
  TicketSlashIcon,
  TicketXIcon,
} from "lucide-react";

// Type alias for components that need basic params
type ChatterboxBasicParams = Omit<ChatterboxParams, "audio_prompt_path"> & {
  [key: string]: string | number | boolean;
};

const SUPPORTED_LANGUAGES = {
  ar: "Arabic 🇸🇦",
  da: "Danish 🇩🇰",
  de: "German 🇩🇪",
  el: "Greek 🇬🇷",
  en: "English 🇬🇧",
  es: "Spanish 🇪🇸",
  fi: "Finnish 🇫🇮",
  fr: "French 🇫🇷",
  he: "Hebrew 🇮🇱",
  hi: "Hindi 🇮🇳",
  it: "Italian 🇮🇹",
  ja: "Japanese 🇯🇵",
  ko: "Korean 🇰🇷",
  ms: "Malay 🇲🇾",
  nl: "Dutch 🇳🇱",
  no: "Norwegian 🇳🇴",
  pl: "Polish 🇵🇱",
  pt: "Portuguese 🇧🇷",
  ru: "Russian 🇷🇺",
  sv: "Swedish 🇸🇪",
  sw: "Swahili 🇰🇪",
  tr: "Turkish 🇹🇷",
  zh: "Chinese 🇨🇳",
};

export const ChatterboxInputs = ({
  chatterboxParams,
  handleChange,
  setChatterboxParams,
  output,
  hyperParams,
}: {
  chatterboxParams: ChatterboxParams;
  handleChange: HandleChange;
  setChatterboxParams: React.Dispatch<React.SetStateAction<ChatterboxParams>>;
  output: React.ReactNode;
  hyperParams: React.ReactNode;
}) => (
  <div className="grid grid-cols-2 gap-4 w-full justify-center">
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle>Text Input</CardTitle>
          <CardDescription>
            Enter the text you want to convert to speech using Chatterbox TTS.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PromptTextArea
            params={chatterboxParams as ChatterboxBasicParams}
            handleChange={handleChange}
            label=""
            name="text"
            rows={4}
            placeholder="Enter text to synthesize..."
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle>Voice Parameters</CardTitle>
          <CardDescription>
            Adjust voice characteristics and generation settings.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="pt-2">
            <Label className="flex items-center gap-2 mb-1">
              Reference Audio:
            </Label>
            <AudioInput
              url={chatterboxParams.audio_prompt_path?.path}
              label="Reference Audio"
              callback={(file) => {
                setChatterboxParams({
                  ...chatterboxParams,
                  audio_prompt_path: toLocalCacheFile(file),
                });
              }}
              filter={["sendToChatterbox"]}
            />
          </div>
          <ParameterSliders
            chatterboxParams={chatterboxParams}
            handleChange={handleChange}
          />
          {/* add voice dropdown */}
          {/* <div className="pt-2">
            <VoiceDropdown
              params={chatterboxParams}
              handleChange={handleChange}
            />
          </div> */}
          <div className="cell">
            <SeedInput params={chatterboxParams} handleChange={handleChange} />
          </div>
        </CardContent>
      </Card>
    </div>
    <div className="flex flex-col gap-4">
      {output}

      <div className="flex justify-start gap-4">
        <div className="grid gap-4 cell w-full">
          <div className="flex justify-between">
            <RadioWithLabel
              label="Model"
              name="model_name"
              inline
              value={chatterboxParams.model_name}
              onChange={handleChange}
              options={[
                ["English", "just_a_placeholder"],
                ["Multilingual", "multilingual"],
              ].map(([text, value]) => ({
                label: text,
                value,
              }))}
            />

            <UnloadModelButton
              onUnload={() =>
                fetch("/api/gradio/chatterbox_unload_model", { method: "POST" })
              }
            />
          </div>
          <SelectWithLabel
            label="Language"
            name="language"
            value={(chatterboxParams as any).language || "en"}
            onChange={handleChange}
            options={Object.entries(SUPPORTED_LANGUAGES).map(
              ([value, label]) => ({
                label,
                value,
              })
            )}
          />

          <div className="flex items-center gap-4 pt-2">
            <DeviceSelect
              value={chatterboxParams.device}
              handleChange={handleChange}
            />
          </div>
          <div className="flex items-center gap-4">
            <DTypeSelect
              value={chatterboxParams.dtype}
              handleChange={handleChange}
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch
              id="cpu_offload"
              checked={chatterboxParams.cpu_offload}
              onCheckedChange={(x) =>
                handleChange({ target: { name: "cpu_offload", value: x } })
              }
            />
            <Label htmlFor="cpu_offload">CPU Offloading</Label>
            <PackageCheckIcon className="w-5 h-5" />
          </div>
        </div>

        {hyperParams}
      </div>

      <ChunkingControls
        chatterboxParams={chatterboxParams}
        handleChange={handleChange}
      />

      <AdvancedSettings
        chatterboxParams={chatterboxParams}
        handleChange={handleChange}
      />
    </div>
  </div>
);

const ParameterSliders = ({
  chatterboxParams,
  handleChange,
}: {
  chatterboxParams: ChatterboxParams;
  handleChange: HandleChange;
}) => {
  return (
    <div className="grid grid-cols-3 gap-2 cell">
      <ParameterSlider
        params={chatterboxParams as ChatterboxBasicParams}
        onChange={handleChange}
        label="Exaggeration"
        name="exaggeration"
        min="0"
        max="2"
        step="0.05"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={chatterboxParams as ChatterboxBasicParams}
        onChange={handleChange}
        label="CFG Weight/Pace"
        name="cfg_weight"
        min="0"
        max="1"
        step="0.05"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={chatterboxParams as ChatterboxBasicParams}
        onChange={handleChange}
        label="Temperature"
        name="temperature"
        min="0.05"
        max="5"
        step="0.05"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
    </div>
  );
};

const ChunkingControls = ({
  chatterboxParams,
  handleChange,
}: {
  chatterboxParams: ChatterboxParams;
  handleChange: HandleChange;
}) => {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Chunking</CardTitle>
        <CardDescription>
          Settings for chunked generation for longer prompts.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex justify-between items-end">
        <div className="space-y-2">
          <div className="flex items-center space-x-2">
            <Switch
              id="chunked"
              checked={chatterboxParams.chunked}
              name="chunked"
              onCheckedChange={(x) =>
                handleChange({ target: { name: "chunked", value: x } })
              }
            />
            <div>
              <Label htmlFor="chunked">Split prompt into chunks</Label>
            </div>
          </div>
          <div className="flex items-center space-x-2">
            <Switch
              id="halve_first_chunk"
              checked={chatterboxParams.halve_first_chunk}
              name="halve_first_chunk"
              onCheckedChange={(x) =>
                handleChange({
                  target: { name: "halve_first_chunk", value: x },
                })
              }
            />
            <div>
              <Label htmlFor="halve_first_chunk">Halve First Chunk size</Label>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <ParameterSlider
            params={chatterboxParams as ChatterboxBasicParams}
            onChange={handleChange}
            label="Desired Length"
            name="desired_length"
            min="10"
            max="1000"
            step="10"
            decimals={0}
          />
          <ParameterSlider
            params={chatterboxParams as ChatterboxBasicParams}
            onChange={handleChange}
            label="Max Length"
            name="max_length"
            min="10"
            max="1000"
            step="10"
            decimals={0}
          />
        </div>
      </CardContent>
    </Card>
  );
};

const AdvancedSettings = ({
  chatterboxParams,
  handleChange,
}: {
  chatterboxParams: ChatterboxParams;
  handleChange: HandleChange;
}) => {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>Advanced Settings</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4">
          <RadioWithLabel
            label="Initial Forward Pass Backend"
            name="initial_forward_pass_backend"
            inline
            value={
              (chatterboxParams as any).initial_forward_pass_backend || "eager"
            }
            onChange={handleChange}
            options={[
              ["Eager", "eager"],
              ["Cudagraphs", "cudagraphs"],
            ].map(([text, value]) => ({
              label: text,
              value,
            }))}
          />
        </div>
        <div className="grid">
          <RadioWithLabel
            label="Generate Token Backend"
            name="generate_token_backend"
            // inline
            variant="horizontal"
            className="gap-4"
            value={
              (chatterboxParams as any).generate_token_backend ||
              "cudagraphs-manual"
            }
            onChange={handleChange}
            options={[
              ["Cudagraphs Manual", "cudagraphs-manual"],
              ["Eager", "eager"],
              ["Cudagraphs", "cudagraphs"],
              ["Inductor", "inductor"],
              ["Cudagraphs Strided", "cudagraphs-strided"],
              ["Inductor Strided", "inductor-strided"],
            ].map(([text, value]) => ({
              label: text,
              value,
            }))}
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <ParameterSlider
            params={chatterboxParams as ChatterboxBasicParams}
            onChange={handleChange}
            label="Max New Tokens"
            name="max_new_tokens"
            min="100"
            max="1000"
            step="10"
            decimals={0}
          />
          <ParameterSlider
            params={chatterboxParams as ChatterboxBasicParams}
            onChange={handleChange}
            label="Max Cache Length"
            name="max_cache_len"
            min="200"
            max="1500"
            step="100"
            decimals={0}
          />
        </div>
      </CardContent>
    </Card>
  );
};

const DTypeSelect = ({
  value,
  handleChange,
}: {
  value: string;
  handleChange: HandleChange;
}) => {
  return (
    <RadioWithLabel
      label="Dtype"
      name="dtype"
      inline
      value={value}
      onChange={handleChange}
      options={[
        {
          label: (
            <div className="flex items-center gap-2">
              <TicketXIcon className="w-5 h-5" />
              <span>Float32</span>
            </div>
          ),
          value: "float32",
        },
        {
          label: (
            <div className="flex items-center gap-2">
              <TicketSlashIcon className="w-5 h-5" />
              <span>Float16</span>
            </div>
          ),
          value: "float16",
        },
        {
          label: (
            <div className="flex items-center gap-2">
              <BrainCogIcon className="w-5 h-5" />
              <span>Bfloat16</span>
            </div>
          ),
          value: "bfloat16",
        },
      ]}
    />
  );
};

const DeviceSelect = ({
  value,
  handleChange,
}: {
  value: string;
  handleChange: HandleChange;
}) => {
  return (
    <RadioWithLabel
      label="Device"
      name="device"
      inline
      value={value}
      onChange={handleChange}
      options={[
        {
          label: (
            <div className="flex items-center gap-2">
              <CableIcon className="w-5 h-5" />
              <span>Auto</span>
            </div>
          ),
          value: "auto",
        },
        {
          label: (
            <div className="flex items-center gap-2">
              <MicrochipIcon className="w-5 h-5" />
              <span>CPU</span>
            </div>
          ),
          value: "cpu",
        },
        {
          label: (
            <div className="flex items-center gap-2">
              <MemoryStickIcon className="w-5 h-5" />
              <span>CUDA</span>
            </div>
          ),
          value: "cuda",
        },
        {
          label: (
            <div className="flex items-center gap-2">
              <CpuIcon className="w-5 h-5" />
              <span>MPS</span>
            </div>
          ),
          value: "mps",
        },
      ]}
    />
  );
};
