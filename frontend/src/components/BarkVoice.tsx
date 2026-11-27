import React, { useState } from "react";
import { BarkGenerationParams } from "../tabs/BarkGenerationParams";
import { BARK_VOICE_TO_TRAIT } from "../data/BARK_VOICE_TO_TRAIT";
import { BARK_LANGUAGE_TO_CODE } from "../data/BARK_LANGUAGE_TO_CODE";
import { HandleChange, HandleChangeEvent } from "../types/HandleChange";
import { SwitchWithLabel } from "./SwitchWithLabel";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { Label } from "./ui/label";
import { Separator } from "./ui/separator";

export const BarkVoice = ({
  barkGenerationParams,
  handleChange: handleChange2,
}: {
  barkGenerationParams: BarkGenerationParams;
  handleChange: HandleChange;
}) => {
  const [language, setLanguage] = useState("English");
  const [speakerID, setSpeakerID] = useState("0");
  const [useV2, setUseV2] = useState(true);

  const updatePrompt = (language: string, speakerID: string, useV2: boolean) =>
    handleChange2({
      target: {
        name: "history_prompt",
        value: `${useV2 ? "v2/" : ""}${
          BARK_LANGUAGE_TO_CODE[language]
        }_speaker_${speakerID}`,
      },
    });

  const handleChange = (e: HandleChangeEvent) => {
    switch (e.target.name) {
      case "language":
        const newLanguage = e.target.value as string;
        setLanguage(newLanguage);
        updatePrompt(newLanguage, speakerID, useV2);
        break;
      case "speakerID":
        const newSpeakerID = e.target.value as string;
        setSpeakerID(newSpeakerID);
        updatePrompt(language, newSpeakerID, useV2);
        break;
      case "useV2":
        const newUseV2 = e.target.value as boolean;
        setUseV2(newUseV2);
        updatePrompt(language, speakerID, newUseV2);
        break;
    }
  };

  return (
    <div className="flex flex-col gap-4 cell">
      <Label>Voice:</Label>
      <RadioWithLabel
        label="Language"
        name="language"
        inline
        value={language}
        onChange={handleChange}
        options={BARK_VOICE_LANGUAGES.map((language) => ({
          label: language,
          value: language,
        }))}
        className="flex-col items-start"
      />
      <Separator />
      <RadioWithLabel
        className="flex-col items-start"
        label="Speaker ID"
        name="speakerID"
        inline
        value={speakerID}
        onChange={handleChange}
        options={SPEAKER_IDS.map((speakerID) => ({
          label: speakerID,
          value: speakerID,
        }))}
      />
      <Separator />
      <div className="flex gap-2 items-center justify-between">
        <SwitchWithLabel
          label="Use V2"
          name="useV2"
          value={useV2}
          onChange={handleChange}
        />
        {BARK_VOICE_TO_TRAIT[barkGenerationParams.history_prompt] && (
          <div className="text-sm">
            Chosen voice: {barkGenerationParams.history_prompt}{", "}
            Gender:{" "}
            {BARK_VOICE_TO_TRAIT[barkGenerationParams.history_prompt] ||
              "Unknown"}
          </div>
        )}
      </div>
    </div>
  );
};

const SPEAKER_IDS = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"];

const BARK_VOICE_LANGUAGES = [
  "English",
  "German",
  "Spanish",
  "French",
  "Hindi",
  "Italian",
  "Japanese",
  "Korean",
  "Polish",
  "Portuguese",
  "Russian",
  "Turkish",
  "Chinese",
];
