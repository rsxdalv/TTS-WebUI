import React from "react";
import { MagnetParams } from "../tabs/MagnetParams";
import { MagnetModelSelector } from "./MagnetModelSelector";
import { ParameterSlider } from "./GenericSlider";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";
import { SeedInput } from "./SeedInput";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { SwitchWithLabel } from "./SwitchWithLabel";
import { Link2Icon, Unlink2Icon } from "lucide-react";

export const MagnetInputs = ({
  magnetParams: params,
  handleChange: onChange,
}: {
  magnetParams: MagnetParams;
  handleChange: HandleChange;
}) => (
  <div className="flex flex-col w-full gap-y-4">
    <div className="flex gap-x-6 w-full justify-center">
      <div className="flex flex-col gap-y-2 w-1/2">
        <MagnetModelSelector magnetParams={params} handleChange={onChange} />
        <div className="flex gap-2 w-full justify-between cell">
          <ParameterSlider
            label="Top-K"
            name="top_k"
            params={params}
            onChange={onChange}
            min="0"
            max="250"
            step="1"
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            label="Top-P"
            decimals={2}
            name="top_p"
            params={params}
            onChange={onChange}
            min="0"
            max="1.5"
            step="0.01"
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            label="Temperature"
            decimals={2}
            name="temperature"
            params={params}
            onChange={onChange}
            min="0"
            max="10"
            step="0.01"
            orientation="vertical"
            className="h-40"
          />
        {/* </div>
        <div className="flex gap-2"> */}
          <ParameterSlider
            label="Min Cfg Coef"
            decimals={1}
            name="min_cfg_coef"
            params={params}
            onChange={onChange}
            min="0"
            max="100"
            step="0.1"
            orientation="vertical"
            className="h-40"
          />
          <ParameterSlider
            label="Max Cfg Coef"
            decimals={1}
            name="max_cfg_coef"
            params={params}
            onChange={onChange}
            min="0"
            max="100"
            step="0.1"
            orientation="vertical"
            className="h-40"
          />
        </div>
      </div>
      <div className="flex flex-col gap-y-4">
        <div className={"flex flex-col gap-y-2 items-center cell"}>
          <label className="text-sm">
            Decoding Steps:{" "}
            {[
              params.decoding_steps_1,
              params.decoding_steps_2,
              params.decoding_steps_3,
              params.decoding_steps_4,
            ].map((value, index) => (
              <span key={index} className="inline-block w-6">
                {value}
              </span>
            ))}
          </label>
          <div className="flex gap-2">
            <ParameterSlider
              label=""
              name="decoding_steps_1"
              params={params}
              onChange={onChange}
              min="0"
              max="100"
              step="1"
              orientation="vertical"
              className="h-40"
            />
            <ParameterSlider
              label=""
              name="decoding_steps_2"
              params={params}
              onChange={onChange}
              min="0"
              max="100"
              step="1"
              orientation="vertical"
              className="h-40"
            />
            <ParameterSlider
              label=""
              name="decoding_steps_3"
              params={params}
              onChange={onChange}
              min="0"
              max="100"
              step="1"
              orientation="vertical"
              className="h-40"
            />
            <ParameterSlider
              label=""
              name="decoding_steps_4"
              params={params}
              onChange={onChange}
              min="0"
              max="100"
              step="1"
              orientation="vertical"
              className="h-40"
            />
          </div>
        </div>

        <RadioWithLabel
          label="Span Arrangement"
          name="span_arrangement"
          inline
          value={params.span_arrangement}
          onChange={onChange}
          options={[
            // { label: "Non-overlap", value: "nonoverlap" },
            {
              label: (
                <div className="flex items-center gap-2">
                  <Unlink2Icon className="w-5 h-5" />
                  <span>Non-overlap</span>
                </div>
              ),
              value: "nonoverlap",
            },
            // { label: "Overlap", value: "overlap" },
            {
              label: (
                <div className="flex items-center gap-2">
                  <Link2Icon className="w-5 h-5" />
                  <span>Overlap</span>
                </div>
              ),
              value: "overlap",
            },
          ]}
        />
        <SwitchWithLabel
          label="Use Sampling"
          name="use_sampling"
          value={params.use_sampling}
          onChange={onChange}
        />
        <SeedInput params={params} handleChange={onChange} />
      </div>
    </div>
    <PromptTextArea
      handleChange={onChange}
      label="Text"
      params={params}
      name="text"
    />
  </div>
);
