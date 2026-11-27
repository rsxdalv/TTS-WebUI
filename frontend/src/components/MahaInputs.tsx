import React from "react";
import { MahaParams } from "../tabs/MahaParams";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";
import { SeedInput } from "./SeedInput";
import { ModelDropdown, UnloadModelButton } from "./component/ModelDropdown";
import { Button } from "./ui/button";
import { RadioWithLabel } from "./component/RadioWithLabel";
import { BotIcon, CpuIcon, KeyboardMusicIcon } from "lucide-react";

const Speaker = ({
  mahaParams,
  handleChange,
}: {
  mahaParams: MahaParams;
  handleChange: HandleChange;
}) => {
  const [options, setOptions] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState<boolean>(false);

  const fetchOptions = async () => {
    setLoading(true);
    const response = await fetch("/api/gradio/maha_tts_refresh_voices", {
      method: "POST",
    });

    const result = await response.json();
    setOptions(result);
    setLoading(false);
  };

  const openVoices = async () => {
    await fetch("/api/gradio/tortoise_open_voices", {
      method: "POST",
    });
  };

  React.useEffect(() => {
    fetchOptions();
  }, []);

  const selected = mahaParams?.speaker_name;
  return (
    <ModelDropdown
      name="speaker_name"
      label="Speaker"
      options={options
        .filter((option) => option !== selected)
        .concat(selected)
        .filter((x) => x !== "")}
      value={selected}
      onChange={handleChange}
      onRefresh={fetchOptions}
      onOpen={openVoices}
      loading={loading}
    />
  );
};

export const MahaInputs = ({
  mahaParams,
  handleChange,
  resetParams,
}: {
  mahaParams: MahaParams;
  handleChange: HandleChange;
  resetParams: () => void;
}) => (
  <div className="flex gap-x-6 w-full justify-center">
    <div className="flex flex-col gap-y-4 w-1/2">
      <PromptTextArea
        params={mahaParams}
        handleChange={handleChange}
        label="Text"
        name="text"
      />
      <div className="flex items-center gap-4">
        <RadioWithLabel
          label="Model"
          name="model_name"
          inline
          value={mahaParams.model_name}
          onChange={handleChange}
          options={[
            ["English", "Smolie-en"],
            ["Indian", "Smolie-in"],
          ].map(([text, model_name]) => ({
            label: text,
            value: model_name,
          }))}
        />
        <UnloadModelButton
          onUnload={() =>
            fetch("/api/gradio/maha_tts_unload_model", { method: "POST" })
          }
        />
      </div>

      <RadioWithLabel
        label="Text Language"
        name="text_language"
        inline
        className="flex-col items-start"
        value={mahaParams.text_language}
        onChange={handleChange}
        options={[
          "english",
          "tamil",
          "telugu",
          "punjabi",
          "marathi",
          "hindi",
          "gujarati",
          "bengali",
          "assamese",
        ].map((text_language) => ({
          label: text_language[0].toUpperCase() + text_language.slice(1),
          value: text_language,
        }))}
      />
    </div>
    <div className="flex flex-col gap-y-4 w-1/2">
      <Speaker mahaParams={mahaParams} handleChange={handleChange} />

      <p className="text-sm">
        Note: The speaker audio must be mono at this time.
      </p>

      <SeedInput params={mahaParams} handleChange={handleChange} />

      <RadioWithLabel
        label="Device"
        name="device"
        inline
        value={mahaParams.device}
        onChange={handleChange}
        options={[
          {
            label: (
              <div className="flex items-center gap-2">
                <BotIcon className="w-5 h-5" />
                <span>auto</span>
              </div>
            ),
            value: "auto",
          },
          {
            label: (
              <div className="flex items-center gap-2">
                <CpuIcon className="w-5 h-5" />
                <span>cpu</span>
              </div>
            ),
            value: "cpu",
          },
          {
            label: (
              <div className="flex items-center gap-2">
                <KeyboardMusicIcon className="w-5 h-5" />
                <span>cuda</span>
              </div>
            ),
            value: "cuda",
          },
        ]}
      />

      <Button variant="outline" onClick={resetParams}>
        Reset Parameters
      </Button>
    </div>
  </div>
);
