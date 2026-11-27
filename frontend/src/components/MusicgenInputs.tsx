import React from "react";
import { AudioInput } from "./AudioComponents";
import {
  MusicgenParams,
  MusicgenResult,
  initialMusicgenParams,
} from "../tabs/MusicgenParams";
import { MusicgenModelSelector } from "./MusicgenModelSelector";
import { ParameterSlider } from "./GenericSlider";
import { PromptTextArea } from "./PromptTextArea";
import { HandleChange } from "../types/HandleChange";
import { SeedInput } from "./SeedInput";
import { SwitchWithLabel } from "./SwitchWithLabel";

export const MusicgenInputs = ({
  musicgenParams,
  handleChange,
  setMusicgenParams,
  musicgenResult,
}: {
  musicgenParams: MusicgenParams;
  handleChange: HandleChange;
  setMusicgenParams: React.Dispatch<React.SetStateAction<MusicgenParams>>;
  musicgenResult?: MusicgenResult | null;
}) => (
  <div className="col-span-2 flex gap-x-6 w-full justify-center">
    <div className="flex flex-col gap-y-2 w-1/2">
      <PromptTextArea
        handleChange={handleChange}
        label="Text"
        params={musicgenParams}
        name="text"
      />

      <MusicgenModelSelector
        musicgenParams={musicgenParams}
        setMusicgenParams={setMusicgenParams}
      />
      <AudioInput
        url={musicgenParams.melody}
        label="Melody"
        callback={(file) => {
          setMusicgenParams({
            ...musicgenParams,
            melody: file,
          });
        }}
        filter={["sendToMusicgen"]}
      />
    </div>

    <div className="flex flex-col gap-4 w-1/2">
      <ParameterSlider
        label="Duration"
        // format={(x) => `${x.toFixed(1)}s ${x > 30 ? "(spliced)" : ""}`}
        decimals={1}
        name="duration"
        min="0.5"
        max="360"
        step="0.5"
        params={musicgenParams}
        onChange={handleChange}
      />

      {/* <div className="flex gap-2 w-full justify-between"> */}
      <div className="grid grid-cols-4 gap-2">
        <ParameterSlider
          label="Top-K"
          name="topk"
          min="0"
          max="250"
          step="1"
          params={musicgenParams}
          onChange={handleChange}
          orientation="vertical"
          className="h-40"
        />
        <ParameterSlider
          label="Top-P"
          decimals={2}
          name="topp"
          min="0"
          max="1.5"
          step="0.01"
          params={musicgenParams}
          onChange={handleChange}
          orientation="vertical"
          className="h-40"
        />
      {/* </div>

      <div className="flex gap-2 w-full justify-between"> */}
        <ParameterSlider
          label="Temperature"
          decimals={2}
          name="temperature"
          min="0"
          max="1.5"
          step="0.01"
          params={musicgenParams}
          onChange={handleChange}
          orientation="vertical"
          className="h-40"
        />

        <ParameterSlider
          // label="Classifier Free Guidance Coefficient"
          label="CFG Coefficient"
          decimals={1}
          name="cfg_coef"
          min="0"
          max="10"
          step="0.1"
          params={musicgenParams}
          onChange={handleChange}
          orientation="vertical"
          className="h-40"
        />
      </div>

      <SwitchWithLabel
        label={
          <>
            Use{" "}
            <a
              className="underline"
              href="https://huggingface.co/facebook/multiband-diffusion"
              target="_blank"
            >
              Multi Band Diffusion (High VRAM Usage):
            </a>
          </>
        }
        name="use_multi_band_diffusion"
        value={musicgenParams.use_multi_band_diffusion}
        onChange={handleChange}
      />

      <SeedInput params={musicgenParams} handleChange={handleChange} />

      <button
        className="cell"
        onClick={() => setMusicgenParams(initialMusicgenParams)}
      >
        Reset Parameters
      </button>
    </div>
  </div>
);
