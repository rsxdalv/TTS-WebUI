import React from "react";
import { HandleChange } from "../types/HandleChange";
import { MiniMaxTTSParams } from "../tabs/MiniMaxTTSParams";
import { PromptTextArea } from "./PromptTextArea";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import { SelectWithLabel } from "./component/SelectWithLabel";

const MINIMAX_MODELS = [
  { label: "speech-2.8-hd (High Quality)", value: "speech-2.8-hd" },
  { label: "speech-2.8-turbo (Fast)", value: "speech-2.8-turbo" },
];

const MINIMAX_VOICES = [
  { label: "English Graceful Lady", value: "English_Graceful_Lady" },
  { label: "English Insightful Speaker", value: "English_Insightful_Speaker" },
  { label: "English Radiant Girl", value: "English_radiant_girl" },
  { label: "English Persuasive Man", value: "English_Persuasive_Man" },
  { label: "English Lucky Robot", value: "English_Lucky_Robot" },
  { label: "Wise Woman", value: "Wise_Woman" },
  { label: "Cute Boy", value: "cute_boy" },
  { label: "Lovely Girl", value: "lovely_girl" },
  { label: "Friendly Person", value: "Friendly_Person" },
  { label: "Inspirational Girl", value: "Inspirational_girl" },
  { label: "Deep Voice Man", value: "Deep_Voice_Man" },
  { label: "Sweet Girl", value: "sweet_girl" },
];

export const MiniMaxTTSInputs = ({
  minimaxTTSParams,
  handleChange,
  output,
  hyperParams,
}: {
  minimaxTTSParams: MiniMaxTTSParams;
  handleChange: HandleChange;
  output: React.ReactNode;
  hyperParams: React.ReactNode;
}) => (
  <div className="grid grid-cols-2 gap-4 w-full justify-center">
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle>Text Input</CardTitle>
          <CardDescription>
            Enter the text you want to convert to speech using MiniMax Cloud TTS.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PromptTextArea
            params={minimaxTTSParams}
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
          <CardTitle>Voice Settings</CardTitle>
          <CardDescription>
            Select the TTS model and voice for generation.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <SelectWithLabel
            label="Model"
            name="model"
            value={minimaxTTSParams.model}
            onChange={handleChange}
            options={MINIMAX_MODELS}
          />
          <SelectWithLabel
            label="Voice"
            name="voice_id"
            value={minimaxTTSParams.voice_id}
            onChange={handleChange}
            options={MINIMAX_VOICES}
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle>API Configuration</CardTitle>
          <CardDescription>
            Optional if MINIMAX_API_KEY environment variable is set.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <input
            type="password"
            name="api_key"
            placeholder="Enter MiniMax API key..."
            value={minimaxTTSParams.api_key || ""}
            onChange={(e) =>
              handleChange({
                target: { name: "api_key", value: e.target.value },
              })
            }
            className="w-full p-2 border rounded bg-background text-foreground"
          />
        </CardContent>
      </Card>
    </div>
    <div className="flex flex-col gap-4">
      {output}
      {hyperParams}
    </div>
  </div>
);
