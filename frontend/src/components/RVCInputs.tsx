import React from "react";
import { AudioInput } from "./AudioComponents";
import { RVCParams } from "../tabs/RVCParams";
import { ParameterSlider } from "./GenericSlider";
import { HandleChange } from "../types/HandleChange";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { ModelDropdown } from "./component/ModelDropdown";
import { Button } from "./ui/button";

const Model = ({
  rvcParams: rvcGenerationParams,
  handleChange,
}: {
  rvcParams: RVCParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState<boolean>(false);

  const fetchOptions = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/rvc_model_reload", {
      method: "POST",
    });

    const result = await response.json();
    setOptions(result);
    setLoading(false);
  };

  const openModels = async () => {
    await fetch("/api/gradio/rvc_model_open", {
      method: "POST",
    });
  };

  React.useEffect(() => {
    fetchOptions();
  }, []);

  const selected = rvcGenerationParams?.model;
  console.log("selected", selected);
  return (
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
      onUnload={() => fetch("/api/gradio/rvc_unload_model", { method: "POST" })}
      loading={loading}
    />
  );
};

// Like model but for Index
const Index = ({
  rvcParams: rvcGenerationParams,
  handleChange,
}: {
  rvcParams: RVCParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState<boolean>(false);

  const fetchOptions = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/rvc_index_reload", {
      method: "POST",
    });

    const result = await response.json();
    setOptions(result);
    setLoading(false);
  };

  const openModels = async () => {
    await fetch("/api/gradio/rvc_index_open", {
      method: "POST",
    });
  };

  React.useEffect(() => {
    fetchOptions();
  }, []);

  const selected = rvcGenerationParams?.index;
  return (
    <ModelDropdown
      name="index"
      label="Index"
      options={options
        .filter((option) => option !== selected)
        .concat(selected)
        .filter((x) => x !== "")}
      value={selected}
      onChange={handleChange}
      onRefresh={fetchOptions}
      onOpen={openModels}
      onUnload={() => fetch("/api/gradio/rvc_unload_model", { method: "POST" })}
      loading={loading}
    />
  );
};

const Device = ({
  rvcParams,
  handleChange,
}: {
  rvcParams: RVCParams;
  handleChange: HandleChange;
}) => (
  <div className="flex gap-2">
    <label className="text-sm">Device:</label>
    <div className="flex gap-x-4">
      {["cuda:0", "cpu", "mps"].map((device) => (
        <div key={device} className="flex items-center">
          <input
            type="radio"
            name="device"
            id={device}
            value={device}
            checked={rvcParams.device === device}
            onChange={handleChange}
            className="cell"
          />
          <label className="ml-1 select-none" htmlFor={device}>
            {device}
          </label>
        </div>
      ))}
    </div>
  </div>
);

export const RVCInputs = ({
  rvcParams,
  handleChange,
  hideAudioInput,
}: {
  rvcParams: RVCParams;
  handleChange: HandleChange;
  hideAudioInput?: boolean;
}) => (
  <div className="flex flex-col gap-y-2 p-2">
    <div className=" flex flex-col gap-y-2 ">
      <Model rvcParams={rvcParams} handleChange={handleChange} />
      <Index rvcParams={rvcParams} handleChange={handleChange} />
    </div>

    {/* <Device rvcParams={rvcParams} handleChange={handleChange} /> */}
    {/* <div className="flex gap-2">
      <label className="text-sm">Use half precision model:</label>
      <input
        type="checkbox"
        name="use_half_precision_model"
        checked={rvcParams?.use_half_precision_model}
        onChange={handleChange}
        className="cell"
      />
    </div> */}
    <div className="grid grid-cols-12 gap-2">
      <RadioWithLabel
        label="Pitch Collection Method"
        name="pitch_collection_method"
        value={rvcParams.pitch_collection_method}
        onChange={handleChange}
        options={[
          { label: "Harvest", value: "harvest" },
          { label: "PM", value: "pm" },
          { label: "Crepe", value: "crepe" },
          { label: "RMVPE", value: "rmvpe" },
          { label: "FCPE", value: "fcpe" },
        ]}
        className="mb-auto"
      />
      <div className="col-span-5 grid grid-cols-5 gap-2 items-end cell">
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          label="Semitone shift"
          name="pitch_up_key"
          min="-24"
          max="24"
          step="1"
          // className="w-full"
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          // label="Search Feature Ratio (accent strength)"
          label="Search Feature Ratio"
          name="search_feature_ratio"
          min="0.0"
          max="1.0"
          step="0.01"
          decimals={2}
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          // label="Filter Radius (Pitch median filtering)"
          label="Filter Radius"
          name="filter_radius_pitch"
          min="0"
          max="10"
          step="1"
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          label="Protect Breath Sounds"
          name="protect_breath_sounds"
          min="0.0"
          max="0.5"
          step="0.01"
          decimals={2}
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          // label="Voice Envelope Normalizaiton (Volume scaling)"
          label="Voice Envelope Normalizaiton"
          name="voice_envelope_normalizaiton"
          min="0.0"
          max="1.0"
          step="0.01"
          decimals={2}
          orientation="vertical"
          className="h-40"
        />
      </div>
      <div className="flex flex-col col-span-6 mb-auto">
        {!hideAudioInput && (
          <AudioInput
            callback={(original_audio?: string) => {
              handleChange({
                target: {
                  name: "original_audio",
                  value: original_audio,
                },
              } as React.ChangeEvent<HTMLInputElement>);
            }}
            label="Original Audio"
            filter={["sendToRvc"]}
            url={rvcParams?.original_audio}
          />
        )}
        <ParameterSlider
          params={rvcParams}
          onChange={handleChange}
          label="Resample to"
          name="resample_sample_rate"
          min="0"
          max="48000"
          step="100"
        />
      </div>
    </div>
  </div>
);
