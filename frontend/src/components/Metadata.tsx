import { Voice } from "../types/Voice";
import AddIcon from "@material-design-icons/svg/filled/add.svg";
import React from "react";
import { MUIIcon } from "./mini/MUIIcon";
import { GenerationRaw } from "../types/Generation";

export const Row = ({
  label,
  value,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
}) => (
  <div className="flex flex-row">
    <div className="font-bold">{label}:</div>
    <div className="ml-1">{value}</div>
  </div>
);

export const Metadata = ({
  prompt,
  language,
  history_hash,
  history_prompt,
  seed,
  text_temp,
  waveform_temp,
  is_big_semantic_model,
  is_big_coarse_model,
  is_big_fine_model,
  hash,
}: Pick<
  GenerationRaw,
  | "prompt"
  | "language"
  | "history_hash"
  | "history_prompt"
  | "seed"
  | "text_temp"
  | "waveform_temp"
  | "is_big_semantic_model"
  | "is_big_coarse_model"
  | "is_big_fine_model"
  | "hash"
>) => (
  <div className="text-xs text-gray-500 flex flex-col w-full break-words">
    <div className="font-bold">Generation details:</div>
    <div className="flex flex-col">
      <Row label="Prompt" value={prompt} />
      <Row
        label="Semantic Model"
        value={is_big_semantic_model ? "Big" : "Small"}
      />
      <Row label="Coarse Model" value={is_big_coarse_model ? "Big" : "Small"} />
      <Row label="Fine Model" value={is_big_fine_model ? "Big" : "Small"} />
      <Row
        label="History Hash"
        value={
          <>
            {history_prompt !== "None" ? history_hash : "None"}
            {isVoicePublished(history_hash) ? (
              <a href={`#voices/${history_hash}`} target="_blank">
                <MUIIcon
                  icon={AddIcon}
                  alt="Add a new voice"
                  className="w-4 h-4 rounded"
                />
              </a>
            ) : (
              ""
            )}
          </>
        }
      />
      {/* <Row label="History Prompt" value={history_prompt} /> */}
      <Row label="History Prompt" value={JSON.stringify(history_prompt)} />
      <Row label="Language" value={language} />
      <Row label="Seed" value={seed} />
      <Row label="Text Temperature" value={text_temp} />
      <Row label="Waveform Temperature" value={waveform_temp} />
      <Row label="Hash" value={hash || "unknown"} />
    </div>
  </div>
);

const isVoicePublished = (history_hash: string) => {
  // const voiceDB = require("../data/voices.json");
  const voiceDB: Record<string, Voice> = {
    "2947e78dd502cc6efc00000000000000": {
      name: "Sample Voice",
      image: "https://i.imgur.com/5M0MvQp.png",
      audio: "",
      download: "",
      author: "none",
      gender: "other",
      language: "american",
      tags: [],
    },
  };
  return history_hash !== "None" && voiceDB[history_hash];
};
