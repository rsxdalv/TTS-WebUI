import React from "react";
import { MagnetParams } from "../tabs/MagnetParams";
import { HandleChange } from "../types/HandleChange";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { ModelDropdown } from "./component/ModelDropdown";
import { modelsFnFactory } from "./modelsFnFactory";
import {
  AudioLinesIcon,
  MusicLargeIcon,
  MusicMediumIcon,
  MusicSmallIcon,
} from "./icons";
import { ClockIcon } from "lucide-react";
import { useEffectOnce } from "@/hooks/useEffectOnce";

const modelToType = {
  "facebook/magnet-small-10secs": "Small",
  "facebook/magnet-medium-10secs": "Medium",
  "facebook/magnet-small-30secs": "Small",
  "facebook/magnet-medium-30secs": "Medium",
  "facebook/audio-magnet-small": "Small",
  "facebook/audio-magnet-medium": "Medium",
};

const computeModel = (type: string, isAudio: boolean, duration: number) => {
  const lowerType = type.toLowerCase();
  const durationSuffix = duration === 30 ? "-30secs" : "-10secs";

  return isAudio
    ? `facebook/audio-magnet-${lowerType}`
    : `facebook/magnet-${lowerType}${durationSuffix}`;
};

const getType = (model: string) => {
  return modelToType[model] || "Small";
};

const decomputeModel = (
  model: string
): { type: string; isAudio: boolean; duration: number } => {
  const type = getType(model);
  const duration = model.includes("-30secs") ? 30 : 10;
  const isAudio = model.includes("audio");
  return { type, isAudio, duration };
};

export const MagnetModelSelector = ({
  magnetParams,
  handleChange,
}: {
  magnetParams: MagnetParams;
  handleChange: HandleChange;
}) => {
  const {
    type: modelType,
    isAudio,
    duration,
  } = decomputeModel(magnetParams.model_name);

  const toModelChange = (newModel: string) =>
    handleChange({ target: { name: "model_name", value: newModel } });

  return (
    <div className="flex flex-col cell text-md gap-2">
      <Model params={magnetParams} handleChange={handleChange} />
      <RadioWithLabel
        inline
        label="Size"
        name="size"
        value={modelType}
        onChange={(event) =>
          toModelChange(computeModel(event.target.value, isAudio, duration))
        }
        options={[
          // { label: "Small", value: "Small" },
          {
            label: (
              <div className="flex items-center gap-2">
                <MusicSmallIcon className="w-5 h-5" />
                <span>Small</span>
              </div>
            ),
            value: "Small",
          },
          // { label: "Medium", value: "Medium" },
          {
            label: (
              <div className="flex items-center gap-2">
                <MusicMediumIcon className="w-5 h-5" />
                <span>Medium</span>
              </div>
            ),
            value: "Medium",
          },
        ]}
      />
      <RadioWithLabel
        inline
        label="Type"
        name="audio"
        value={isAudio ? "Audio" : "Music"}
        onChange={(event) =>
          toModelChange(
            computeModel(modelType, event.target.value === "Audio", duration)
          )
        }
        options={[
          // { label: "Audio", value: "Audio" },
          {
            label: (
              <div className="flex items-center gap-2">
                <AudioLinesIcon className="w-5 h-5" />
                <span>Audio</span>
              </div>
            ),
            value: "Audio",
          },
          // { label: "Music", value: "Music" },
          {
            label: (
              <div className="flex items-center gap-2">
                <MusicLargeIcon className="w-5 h-5" />
                <span>Music</span>
              </div>
            ),
            value: "Music",
          },
        ]}
      />
      <RadioWithLabel
        inline
        label="Duration"
        name="duration"
        value={duration}
        disabled={isAudio}
        onChange={(event) =>
          toModelChange(
            computeModel(modelType, isAudio, Number(event.target.value))
          )
        }
        options={[
          // { label: "10s", value: 10 },
          {
            label: (
              <div className="flex items-center gap-2">
                <ClockIcon className="w-5 h-5" />
                <span>10s</span>
              </div>
            ),
            value: 10,
          },
          // { label: "30s", value: 30 },
          {
            label: (
              <div className="flex items-center gap-2">
                <ClockIcon className="w-5 h-5 rotate-90" />
                <span>30s</span>
              </div>
            ),
            value: 30,
          },
        ]}
      />
    </div>
  );
};

const Model = ({
  params,
  handleChange,
}: {
  params: MagnetParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState<boolean>(false);

  const { fetchOptions, openModels, unloadModel } = modelsFnFactory(
    setLoading,
    setOptions,
    "magnet"
  );

  useEffectOnce(() => {
    fetchOptions();
  });

  const selected = params?.model_name;
  return (
    <ModelDropdown
      name="model_name"
      label="Model"
      options={options.filter((option) => option !== selected).concat(selected)}
      value={selected}
      onChange={handleChange}
      onRefresh={fetchOptions}
      onOpen={openModels}
      onUnload={unloadModel}
      loading={loading}
    />
  );
};
