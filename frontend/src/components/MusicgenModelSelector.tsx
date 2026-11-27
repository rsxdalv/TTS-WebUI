import React from "react";
import { MusicgenParams } from "../tabs/MusicgenParams";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { SwitchWithLabel } from "./SwitchWithLabel";
import { ModelDropdown } from "./component/ModelDropdown";
import { HandleChangeEvent } from "../types/HandleChange";
import { modelsFnFactory } from "./modelsFnFactory";
import {
  MusicSmallIcon,
  MusicMediumIcon,
  MusicLargeIcon,
  AudioLinesIcon,
  MonitorSpeakerIcon,
  FileAudio2Icon,
} from "./icons";
import { Label } from "./ui/label";
import { useEffectOnce } from "../hooks/useEffectOnce";

const modelMap = {
  Small: { size: "small", stereo: true, melody: false },
  Medium: { size: "medium", stereo: true, melody: true },
  Large: { size: "large", stereo: true, melody: true },
  Audiogen: { size: "audiogen", stereo: false, melody: false },
};

const modelToType = {
  "facebook/musicgen-small": "Small",
  "facebook/musicgen-medium": "Medium",
  "facebook/musicgen-large": "Large",
  "facebook/audiogen-medium": "Audiogen",

  "facebook/musicgen-melody": "Medium",
  "facebook/musicgen-melody-large": "Large",

  "facebook/musicgen-stereo-small": "Small",
  "facebook/musicgen-stereo-medium": "Medium",
  "facebook/musicgen-stereo-large": "Large",

  "facebook/musicgen-stereo-melody": "Medium",
  "facebook/musicgen-stereo-melody-large": "Large",
};

const computeModel = (type: string, stereo: boolean, melody: boolean) => {
  const lowerType = type.toLowerCase();
  const largeSuffix = type === "Large" ? "-large" : "";
  const stereoPrefix = stereo ? "-stereo" : "";

  return type === "Audiogen"
    ? `facebook/audiogen-medium`
    : melody && type !== "Small"
      ? `facebook/musicgen${stereoPrefix}-melody${largeSuffix}`
      : `facebook/musicgen${stereoPrefix}-${lowerType}`;
};

const getType = (model: string) => {
  return modelToType[model] || "Small";
};

const decomputeModel = (
  model: string
): { type: string; stereo: boolean; melody: boolean } => {
  const type = getType(model);
  const stereo = model.includes("stereo");
  const melody = model.includes("melody");
  return { type, stereo, melody };
};

export const MusicgenModelSelector = ({
  musicgenParams,
  setMusicgenParams,
}: {
  musicgenParams: MusicgenParams;
  setMusicgenParams: React.Dispatch<React.SetStateAction<MusicgenParams>>;
}) => {
  const {
    type: modelType,
    stereo,
    melody,
  } = decomputeModel(musicgenParams.model_name);

  const { stereo: stereoAvailable, melody: melodyAvailable } =
    modelMap[modelType];

  // const options = Object.keys(modelToType);
  const [options, setOptions] = React.useState<string[]>(
    Object.keys(modelToType)
  );
  const [loading, setLoading] = React.useState<boolean>(false);

  const { fetchOptions, openModels, unloadModel } = modelsFnFactory(
    setLoading,
    setOptions,
    "musicgen_audiogen"
  );

  useEffectOnce(() => {
    fetchOptions();
  });

  const selected = musicgenParams?.model_name;

  return (
    <div className="flex flex-col gap-4 cell text-md">
      <ModelDropdown
        name="model_name"
        label="Model"
        options={options
          .filter((option) => option !== selected)
          .concat(selected)}
        value={selected}
        onChange={(event) =>
          setMusicgenParams({
            ...musicgenParams,
            model_name: event.target.value as string,
          })
        }
        onRefresh={fetchOptions}
        onOpen={openModels}
        onUnload={unloadModel}
        loading={loading}
      />
      <RadioWithLabel
        inline
        label="Size"
        name="size"
        value={modelType}
        onChange={(event) =>
          setMusicgenParams({
            ...musicgenParams,
            model_name: computeModel(event.target.value, stereo, melody),
          })
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
          // { label: "Large", value: "Large" },
          {
            label: (
              <div className="flex items-center gap-2">
                <MusicLargeIcon className="w-5 h-5" />
                <span>Large</span>
              </div>
            ),
            value: "Large",
          },
          // { label: "Audiogen", value: "Audiogen" },
          {
            label: (
              <div className="flex items-center gap-2">
                <AudioLinesIcon className="w-5 h-5" />
                <span>Audiogen</span>
              </div>
            ),
            value: "Audiogen",
          },
        ]}
      />
      <div className="flex gap-4 items-center">
        <Label>Features: </Label>
        <SwitchWithLabel
          // label="Stereo"
          label={
            <div className="flex items-center gap-2">
              <MonitorSpeakerIcon className="w-5 h-5" />
              <span>Stereo</span>
            </div>
          }
          name="stereo"
          value={stereo}
          disabled={!stereoAvailable}
          onChange={() =>
            setMusicgenParams({
              ...musicgenParams,
              model_name: computeModel(modelType, !stereo, melody),
            })
          }
        />

        <SwitchWithLabel
          // label="Melody"
          label={
            <div className="flex items-center gap-2">
              <FileAudio2Icon className="w-5 h-5" />
              <span>Melody</span>
            </div>
          }
          name="melody"
          value={melody}
          disabled={!melodyAvailable}
          onChange={() =>
            setMusicgenParams({
              ...musicgenParams,
              model_name: computeModel(modelType, stereo, !melody),
            })
          }
        />
      </div>
    </div>
  );
};
