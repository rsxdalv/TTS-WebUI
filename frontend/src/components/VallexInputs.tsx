import React from "react";
import {
  VallexParams,
  VallexResult,
  initialVallexParams,
} from "../tabs/VallexParams";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";
import { ResetButton } from "./ResetButton";
import { SeedInput } from "./SeedInput";
import { Textarea } from "./ui/textarea";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { Button } from "./ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";
import { ChevronDownIcon } from "lucide-react";

const VALLEX_LANGUAGE_DATA: [string, string][] = [
  ["Mix", "Mix"],
  ["English", "English"],
  ["Chinese 中文", "中文"],
  ["Japanese 日本語", "日本語"],
];

const VALLEX_ACCENT_DATA: [string, string][] = [
  ["No Accent", "no-accent"],
  ["English", "English"],
  ["Chinese 中文", "中文"],
  ["Japanese 日本語", "日本語"],
];

const VALLEX_MODE_DATA: [string, string][] = [
  ["Short", "short"],
  ["Fixed Prompt", "fixed-prompt"],
  ["Sliding Window", "sliding-window"],
];

const TokenizeButton = ({ params }: { params: VallexParams }) => {
  const [result, setResult] = React.useState<string>("");
  const [loading, setLoading] = React.useState<boolean>(false);

  const tokenize = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/vall_e_x_tokenize", {
      method: "POST",
      body: JSON.stringify({
        text: params.text,
        language: params.language,
      }),
    });
    const result = await response.json();
    setResult(result.tokens);
    setLoading(false);
  };

  return (
    <div className="flex flex-col gap-y-2">
      <Button variant="outline" onClick={tokenize}>
        Preview Tokens
      </Button>
      <div className="flex flex-col gap-y-2">
        {loading && <p>Loading...</p>}
        <Textarea value={result} readOnly rows={2} />
      </div>
    </div>
  );
};

const SplitTextIntoSentences = ({ params }: { params: VallexParams }) => {
  const [result, setResult] = React.useState<string>("");
  const [loading, setLoading] = React.useState<boolean>(false);
  const splitText = async () => {
    setLoading(true);
    const response = await fetch(
      "/api/gradio/vall_e_x_split_text_into_sentences",
      {
        method: "POST",
        body: JSON.stringify({
          text: params.text,
        }),
      }
    );
    const result = await response.json();
    setResult(result.split_text);
    setLoading(false);
  };
  return (
    <div className="flex flex-col gap-y-2">
      <Button variant="outline" onClick={splitText}>
        Preview sentences
      </Button>
      <div className="flex flex-col gap-y-2">
        {loading && <p>Loading...</p>}
        <Textarea value={result} readOnly rows={2} />
      </div>
    </div>
  );
};

export const VallexInputs = ({
  vallexParams: params,
  handleChange,
  setVallexParams,
}: {
  vallexParams: VallexParams;
  handleChange: HandleChange;
  setVallexParams: React.Dispatch<React.SetStateAction<VallexParams>>;
  data?: VallexResult | null;
}) => (
  <div className="flex gap-x-6 w-full justify-center">
    <div className="flex flex-col gap-y-2 w-1/2">
      <PromptTextArea
        params={params}
        handleChange={handleChange}
        label="Text"
        name="text"
      />

      <TokenizeButton params={params} />
      <SplitTextIntoSentences params={params} />
    </div>

    <div className="flex flex-col gap-y-2 w-1/2">
      <RadioWithLabel
        label="Language"
        name="language"
        variant="horizontal"
        value={params.language}
        onChange={handleChange}
        options={VALLEX_LANGUAGE_DATA.map(([label, value]) => ({
          label,
          value,
        }))}
      />

      <RadioWithLabel
        label="Accent"
        name="accent"
        variant="horizontal"
        value={params.accent}
        onChange={handleChange}
        options={VALLEX_ACCENT_DATA.map(([label, value]) => ({
          label,
          value,
        }))}
      />

      <Collapsible className="text-sm">
        <CollapsibleTrigger className="hover:underline flex items-center text-left">
          For longer audio generation, two extension modes are available:
          <ChevronDownIcon className="ml-2 h-4 w-4" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <p>
            <em>(Default) short:</em> This will only generate as long as the
            model's context length.
            <br />
            <em>fixed-prompt:</em> This mode will keep using the same prompt the
            user has provided, and generate audio sentence by sentence.
            <br />
            <em>sliding-window:</em> This mode will use the last sentence as the
            prompt for the next sentence, but has some concern on speaker
            maintenance.
          </p>
        </CollapsibleContent>
      </Collapsible>
      <RadioWithLabel
        label="Long Audio Generation Mode"
        name="mode"
        variant="horizontal"
        value={params.mode}
        onChange={handleChange}
        options={VALLEX_MODE_DATA.map(([label, value]) => ({
          label,
          value,
        }))}
      />

      <SeedInput params={params} handleChange={handleChange} />

      <ResetButton
        params={params}
        setParams={setVallexParams}
        initialParams={initialVallexParams}
      />
    </div>
  </div>
);
