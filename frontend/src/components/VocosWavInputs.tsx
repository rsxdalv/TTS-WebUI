import React from "react";
import { VocosParams } from "../tabs/VocosParams";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { HandleChange } from "../types/HandleChange";

export const VocosWavInputs = ({
  vocosParams,
  handleChange,
}: {
  vocosParams: VocosParams;
  handleChange: HandleChange;
}) => (
  <RadioWithLabel
    label="Bandwith in kbps"
    name="bandwidth"
    inline
    value={vocosParams.bandwidth}
    onChange={handleChange}
    options={[
      { label: "1.5", value: "1.5" },
      { label: "3.0", value: "3.0" },
      { label: "6.0", value: "6.0" },
      { label: "12.0", value: "12.0" },
    ]}
  />
);
