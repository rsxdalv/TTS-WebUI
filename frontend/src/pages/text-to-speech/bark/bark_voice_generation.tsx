import React from "react";
import { Template } from "../../../components/Template";
import useLocalStorage from "../../../hooks/useLocalStorage";
import { AudioInput, AudioOutput } from "../../../components/AudioComponents";
import {
  BarkVoiceGenerationParams,
  barkVoiceGenerationId,
  initialState,
} from "../../../tabs/BarkVoiceGenerationParams";
import { GradioFile } from "../../../types/GradioFile";
import { SelectWithLabel } from "../../../components/component/SelectWithLabel";
import { HandleChangeEvent } from "../../../types/HandleChange";
import { SwitchWithLabel } from "../../../components/SwitchWithLabel";
import { Button } from "../../../components/ui/button";
import { sendToBarkAsVoice } from "../../../tabs/BarkGenerationParams";

type Result = {
  filename: string;
  preview: GradioFile;
};

const BarkVoiceGenerationPage = () => {
  const [data, setData] = useLocalStorage<Result | null>(
    "barkVoiceGenerationOutput",
    null
  );
  const [barkVoiceGenerationParams, setBarkVoiceGenerationParams] =
    useLocalStorage<BarkVoiceGenerationParams>(
      barkVoiceGenerationId,
      initialState
    );

  async function barkVoiceGeneration() {
    const response = await fetch("/api/gradio/bark_voice_generate", {
      method: "POST",
      body: JSON.stringify(barkVoiceGenerationParams),
    });

    const result = await response.json();
    setData(result);
  }

  async function barkVoiceTokenizerLoad(
    barkVoiceGenerationParams: BarkVoiceGenerationParams
  ) {
    const response = await fetch("/api/gradio/bark_voice_tokenizer_load", {
      method: "POST",
      body: JSON.stringify(barkVoiceGenerationParams),
    });

    await response.json();
  }

  const useAsVoice = (_audio?: string, metadata?: Result) => {
    const voice = metadata?.filename;
    if (!voice) return;
    sendToBarkAsVoice(voice);
  };

  return (
    <Template title="Bark Voice Generation">
      <div className="flex gap-x-4">
        <div className="flex flex-col gap-y-2">
          <SelectWithLabel
            name="tokenizer"
            label="Tokenizer"
            options={[
              {
                label:
                  "quantifier_hubert_base_ls960.pth @ GitMylo/bark-voice-cloning",
                value:
                  "quantifier_hubert_base_ls960.pth @ GitMylo/bark-voice-cloning",
              },
              {
                label:
                  "quantifier_hubert_base_ls960_14.pth @ GitMylo/bark-voice-cloning",
                value:
                  "quantifier_hubert_base_ls960_14.pth @ GitMylo/bark-voice-cloning",
              },
              {
                label:
                  "quantifier_V1_hubert_base_ls960_23.pth @ GitMylo/bark-voice-cloning",
                value:
                  "quantifier_V1_hubert_base_ls960_23.pth @ GitMylo/bark-voice-cloning",
              },
              {
                label:
                  "polish-HuBERT-quantizer_8_epoch.pth @ Hobis/bark-voice-cloning-polish-HuBERT-quantizer",
                value:
                  "polish-HuBERT-quantizer_8_epoch.pth @ Hobis/bark-voice-cloning-polish-HuBERT-quantizer",
              },
              {
                label:
                  "german-HuBERT-quantizer_14_epoch.pth @ CountFloyd/bark-voice-cloning-german-HuBERT-quantizer",
                value:
                  "german-HuBERT-quantizer_14_epoch.pth @ CountFloyd/bark-voice-cloning-german-HuBERT-quantizer",
              },
              {
                label: "es_tokenizer.pth @ Lancer1408/bark-es-tokenizer",
                value: "es_tokenizer.pth @ Lancer1408/bark-es-tokenizer",
              },
              {
                label:
                  "portuguese-HuBERT-quantizer_24_epoch.pth @ MadVoyager/bark-voice-cloning-portuguese-HuBERT-quantizer",
                value:
                  "portuguese-HuBERT-quantizer_24_epoch.pth @ MadVoyager/bark-voice-cloning-portuguese-HuBERT-quantizer",
              },
            ]}
            value={barkVoiceGenerationParams?.tokenizer}
            onChange={(e: HandleChangeEvent) => {
              const newParams = {
                ...barkVoiceGenerationParams,
                [e.target.name]: e.target.value,
              };
              setBarkVoiceGenerationParams(newParams);
              barkVoiceTokenizerLoad(newParams);
            }}
          />
          <SwitchWithLabel
            label="Use GPU"
            name="use_gpu"
            value={barkVoiceGenerationParams.use_gpu}
            onChange={(e: HandleChangeEvent) => {
              setBarkVoiceGenerationParams({
                ...barkVoiceGenerationParams,
                [e.target.name]: e.target.value,
              });
            }}
          />
          <AudioInput
            url={barkVoiceGenerationParams?.audio}
            callback={(file) => {
              setBarkVoiceGenerationParams({
                ...barkVoiceGenerationParams,
                audio: file,
              });
            }}
            filter={["sendToBarkVoiceGeneration"]}
          />

          <Button variant="default" onClick={barkVoiceGeneration}>
            Generate Bark Voice
          </Button>
        </div>
        <div className="flex flex-col gap-y-4">
          {/* Print voice filename */}
          <div className="gap-y-2">
            <label className="text-sm">
              Result voice file name (read only):
            </label>
            <input
              type="text"
              name="filename"
              id="filename"
              className="cell text-black w-full"
              value={data?.filename}
              readOnly
            />
          </div>

          <AudioOutput
            audioOutput={data?.preview}
            label="Encodec audio preview"
            funcs={{ useAsVoice }}
            filter={["sendToBarkVoiceGeneration"]}
            metadata={data}
          />
        </div>
      </div>
    </Template>
  );
};

export default BarkVoiceGenerationPage;
