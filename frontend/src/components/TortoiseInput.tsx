import React, { useState, useEffect } from "react";
import { TortoiseGenerationParams } from "../tabs/TortoiseGenerationParams";
import { TortoiseResult } from "../tabs/TortoiseResult";
import { ParameterSlider } from "./GenericSlider";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";
import { SeedInput } from "./SeedInput";
import { splitAndRecombineText } from "../data/split-and-recombine";
import { SwitchWithLabel } from "./SwitchWithLabel";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { ModelDropdown } from "./component/ModelDropdown";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { Separator } from "./ui/separator";
import {
  ActivityIcon,
  AudioLinesIcon,
  AudioWaveformIcon,
  CircleCheckBigIcon,
  MinusIcon,
  RefreshCwIcon,
  ServerOffIcon,
  WrapTextIcon,
} from "lucide-react";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { SingleFileUpload } from "./SingleFileUpload";

const SimpleGroup = ({ ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    {...props}
    className={cn("flex flex-col gap-y-2 cell", props.className)}
  />
);

const presets = {
  ultra_fast: {
    num_autoregressive_samples: 16,
    diffusion_iterations: 30,
    cond_free: false,
  },
  fast: {
    num_autoregressive_samples: 96,
    diffusion_iterations: 80,
  },
  standard: {
    num_autoregressive_samples: 256,
    diffusion_iterations: 200,
  },
  high_quality: {
    num_autoregressive_samples: 256,
    diffusion_iterations: 400,
  },
};

export const TortoiseInput = ({
  tortoiseGenerationParams,
  setTortoiseGenerationParams,
  handleChange,
  data,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  setTortoiseGenerationParams: React.Dispatch<
    React.SetStateAction<TortoiseGenerationParams>
  >;
  handleChange: HandleChange;
  data?: TortoiseResult | null;
}) => {
  const handlePresets = ({ target: { value: preset } }) => {
    const { num_autoregressive_samples, diffusion_iterations, cond_free } =
      presets[preset];
    handleChange({ target: { name: "preset", value: preset } });
    handleChange({
      target: {
        name: "samples",
        value: num_autoregressive_samples,
      },
    });
    handleChange({
      target: {
        name: "diffusion_iterations",
        value: diffusion_iterations,
      },
    });
    handleChange({
      target: { name: "cond_free", value: cond_free ?? true },
    });
  };

  return (
    <div className="flex flex-row gap-x-2">
      <div className="flex flex-col gap-y-2">
        <SimpleGroup>
          <Model
            tortoiseGenerationParams={tortoiseGenerationParams}
            handleChange={handleChange}
          />
        </SimpleGroup>
        <SimpleGroup>
          <Speaker
            tortoiseGenerationParams={tortoiseGenerationParams}
            handleChange={handleChange}
          />
        </SimpleGroup>
        <SimpleGroup>
          <RadioWithLabel
            label="Preset"
            name="preset"
            inline
            value={tortoiseGenerationParams.preset}
            onChange={handlePresets}
            options={[
              {
                label: (
                  <div className="flex items-center gap-2">
                    <MinusIcon className="w-5 h-5" />
                    <span>Ultra Fast</span>
                  </div>
                ),
                value: "ultra_fast",
              },
              {
                label: (
                  <div className="flex items-center gap-2">
                    <ActivityIcon className="w-5 h-5" />
                    <span>Fast</span>
                  </div>
                ),
                value: "fast",
              },
              {
                label: (
                  <div className="flex items-center gap-2">
                    <AudioWaveformIcon className="w-5 h-5" />
                    <span>Standard</span>
                  </div>
                ),
                value: "standard",
              },
              {
                label: (
                  <div className="flex items-center gap-2">
                    <AudioLinesIcon className="w-5 h-5" />
                    <span>High Quality</span>
                  </div>
                ),
                value: "high_quality",
              },
            ]}
          />
        </SimpleGroup>
        <AutoRegressiveParameters
          tortoiseGenerationParams={tortoiseGenerationParams}
          handleChange={handleChange}
        />
        <PromptTextArea
          name="text"
          label="Prompt"
          handleChange={handleChange}
          params={tortoiseGenerationParams}
          optimalLength={200}
          softMaxLength={300}
          hardMaxLength={500}
        />
      </div>
      <div className="flex flex-col gap-y-2">
        <DiffusionParameters
          tortoiseGenerationParams={tortoiseGenerationParams}
          handleChange={handleChange}
        />
        <GenerationParameters
          tortoiseGenerationParams={tortoiseGenerationParams}
          handleChange={handleChange}
        />
      </div>
    </div>
  );
};

const Speaker = ({
  tortoiseGenerationParams,
  handleChange,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = useState<string[]>([]);
  const [loading, setLoading] = useState<boolean>(false);

  const fetchOptions = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/tortoise_refresh_voices", {
      method: "POST",
    });

    const result = await response.json();
    setOptions(result);
    setLoading(false);
  };

  const openVoices = async () => {
    await fetch("/api/gradio/tortoise_open_voices", {
      method: "POST",
    });
  };

  React.useEffect(() => {
    fetchOptions();
  }, []);

  const selected = tortoiseGenerationParams?.speaker;
  return (
    <ModelDropdown
      name="speaker"
      label="Speaker"
      options={options}
      value={selected}
      onChange={handleChange}
      onRefresh={fetchOptions}
      onOpen={openVoices}
      loading={loading}
    />
  );
};

const Model = ({
  tortoiseGenerationParams,
  handleChange,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = useState<string[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [applyModelSettingsLoading, setApplyModelSettingsLoading] =
    useState<boolean>(false);

  const fetchOptions = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/tortoise_refresh_models", {
      method: "POST",
    });

    const result = await response.json();
    setOptions(result);
    setLoading(false);
  };

  const openModels = async () => {
    await fetch("/api/gradio/tortoise_open_models", {
      method: "POST",
    });
  };

  const applyModelSettings = async () => {
    setApplyModelSettingsLoading(true);
    const params = {
      model: tortoiseGenerationParams.model,
      kv_cache: tortoiseGenerationParams.kv_cache,
      use_deepspeed: tortoiseGenerationParams.use_deepspeed,
      half: tortoiseGenerationParams.half,
      tokenizer: tortoiseGenerationParams.tokenizer,
      use_basic_cleaners: tortoiseGenerationParams.use_basic_cleaners,
    };
    const response = await fetch("/api/gradio/tortoise_apply_model_settings", {
      method: "POST",
      body: JSON.stringify(params),
    });
    const result = await response.json();
    console.log(result);
    setApplyModelSettingsLoading(false);
  };

  const BasicSwitch = ({ name, label }: { name: string; label: string }) => (
    <SwitchWithLabel
      label={label}
      name={name}
      value={tortoiseGenerationParams[name]}
      onChange={handleChange}
    />
  );

  useEffect(() => {
    fetchOptions();
  }, []);

  const selected = tortoiseGenerationParams?.model;
  return (
    <div className="flex flex-col gap-y-4">
      <ModelDropdown
        name="model"
        label="Model"
        options={options
          .filter((option) => option !== selected)
          .concat(selected)
          .filter((x) => x !== "")}
        value={selected}
        onChange={handleChange}
        onRefresh={fetchOptions}
        onOpen={openModels}
        onUnload={() =>
          fetch("/api/gradio/tortoise_unload_model", { method: "POST" })
        }
        loading={loading}
      />

      <div className="flex gap-4 justify-between">
        <div className="w-52 flex flex-col gap-4 my-auto">
          <BasicSwitch name="kv_cache" label="KV Cache" />
          <BasicSwitch name="use_deepspeed" label="Deepspeed" />
          <BasicSwitch name="half" label="Half precision" />
          <BasicSwitch name="use_basic_cleaners" label="Basic Cleaners" />
        </div>
        <div className="w-full mb-auto order-2 gap-2 flex flex-col">
          <SingleFileUpload
            label="Tokenizer"
            file={tortoiseGenerationParams.tokenizer}
            callback={(tokenizer) => {
              handleChange({ target: { name: "tokenizer", value: tokenizer } });
            }}
          />
          <Button onClick={applyModelSettings} variant="outline">
            {applyModelSettingsLoading ? "Applying..." : "Apply Model Settings"}
            {applyModelSettingsLoading ? (
              <RefreshCwIcon className="ml-2 w-5 h-5 animate-spin" />
            ) : (
              <CircleCheckBigIcon className="ml-2 w-5 h-5" />
            )}
          </Button>
        </div>
      </div>
    </div>
  );
};

const AutoRegressiveParameters = ({
  tortoiseGenerationParams,
  handleChange,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  handleChange: HandleChange;
}) => (
  <div className="flex flex-col gap-y-0">
    <h2 className="text-md">Autoregressive Parameters</h2>
    <div className="grid grid-cols-6 gap-2 cell items-end">
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Samples"
        name="samples"
        // min="1" - needs to be at least GPU batch sized
        min="4"
        max="256"
        step="1"
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Temperature"
        name="temperature"
        min="0.0"
        max="1.0"
        step="0.01"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Length Penalty"
        name="length_penalty"
        min="0.0"
        max="10.0"
        step="0.01"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Repetition Penalty"
        name="repetition_penalty"
        min="0.0"
        max="10.0"
        step="0.01"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Top P"
        name="top_p"
        min="0.0"
        max="1.0"
        step="0.01"
        decimals={2}
        orientation="vertical"
        className="h-40"
      />
      <ParameterSlider
        params={tortoiseGenerationParams}
        onChange={handleChange}
        label="Max Mel Tokens"
        name="max_mel_tokens"
        min="0"
        max="600"
        step="1"
        orientation="vertical"
        className="h-40"
      />
    </div>
  </div>
);

const DiffusionParameters = ({
  tortoiseGenerationParams,
  handleChange,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  handleChange: HandleChange;
}) => (
  <div className="flex flex-col gap-y-0">
    <h2 className="text-md">Diffusion Parameters</h2>
    <SimpleGroup>
      <div className="flex gap-2 w-full justify-around">
        <ParameterSlider
          params={tortoiseGenerationParams}
          onChange={handleChange}
          label="Diffusion Iterations"
          name="diffusion_iterations"
          min="0"
          max="400"
          step="1"
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={tortoiseGenerationParams}
          onChange={handleChange}
          label="Cond Free K"
          name="cond_free_k"
          min="0"
          max="10"
          step="1"
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={tortoiseGenerationParams}
          onChange={handleChange}
          label="Diffusion Temperature"
          decimals={2}
          name="diffusion_temperature"
          min="0.0"
          max="1.0"
          step="0.01"
          orientation="vertical"
          className="h-40"
        />
      </div>
      <Separator />
      <SwitchWithLabel
        label="Use Cond Free"
        name="cond_free"
        value={tortoiseGenerationParams.cond_free}
        onChange={handleChange}
      />
    </SimpleGroup>
  </div>
);

function GenerationParameters({
  tortoiseGenerationParams,
  handleChange,
}: {
  tortoiseGenerationParams: TortoiseGenerationParams;
  handleChange: HandleChange;
}) {
  return (
    <div className="flex flex-col gap-y-0">
      <h2 className="text-md">Generation Parameters</h2>
      <SimpleGroup>
        {/* <ParameterSlider
            params={tortoiseGenerationParams}
            onChange={handleChange}
            label="CVVP Amount (Deprecated, always 0)"
            name="cvvp_amount"
            min="0.0"
            max="1.0"
            step="0.01"
            disabled
          /> */}
        <SwitchWithLabel
          label="Generate each line separately"
          name="split_prompt"
          value={tortoiseGenerationParams.split_prompt}
          onChange={handleChange}
        />
        <Button
          className="cell"
          variant="outline"
          onClick={() => {
            handleChange({
              target: {
                name: "text",
                value: splitAndRecombineText(
                  tortoiseGenerationParams.text
                ).join("\n"),
              },
            });
            handleChange({
              target: {
                name: "split_prompt",
                value: true as any,
              },
            });
          }}
        >
          Split prompt into lines by length
          <WrapTextIcon className="ml-2 w-5 h-5" />
        </Button>
        <Separator />
        <ParameterSlider
          params={tortoiseGenerationParams}
          onChange={handleChange}
          label="Candidates"
          name="candidates"
          min="1"
          max="9"
          step="1"
        />
        <Separator />
        <SeedInput
          params={tortoiseGenerationParams}
          handleChange={handleChange}
        />
        <Separator />
        <div className="flex items-center gap-x-2">
          <Label>Generation name:</Label>
          <Input
            type="text"
            name="generation_name"
            value={tortoiseGenerationParams.generation_name}
            onChange={handleChange}
          />
        </div>
      </SimpleGroup>
    </div>
  );
}
