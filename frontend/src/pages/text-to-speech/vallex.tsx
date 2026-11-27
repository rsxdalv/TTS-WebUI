import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { useVallexPage } from "../../tabs/VallexParams";
import { HyperParameters } from "../../components/HyperParameters";
import { VallexInputs } from "../../components/VallexInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "../../components/ui/collapsible";
import { ChevronDownIcon } from "lucide-react";
import { Separator } from "../../components/ui/separator";

const VallexPage = () => {
  const {
    historyData,
    setHistoryData,
    vallexParams,
    setVallexParams,
    consumer: vallexConsumer,
    funcs,
    handleChange,
  } = useVallexPage();

  return (
    <Template title="Vallex">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <Collapsible>
          <CollapsibleTrigger className="hover:underline flex items-center">
            Description
            <ChevronDownIcon className="ml-2 h-4 w-4" />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <p>
              VALL-E X is an amazing multilingual text-to-speech (TTS) model
              proposed by Microsoft. While Microsoft initially publish in their
              research paper, they did not release any code or pretrained
              models. Recognizing the potential and value of this technology,
              our team took on the challenge to reproduce the results and train
              our own model. We are glad to share our trained VALL-E X model
              with the community, allowing everyone to experience the power
              next-generation TTS! 🎧
            </p>
            <Separator />
            <p>
              Implemention GitHub page:{" "}
              <a
                href="https://github.com/Plachtaa/VALL-E-X"
                className="hover:underline text-blue-500"
                target="_blank"
                rel="noreferrer"
              >
                https://github.com/Plachtaa/VALL-E-X
              </a>
            </p>
          </CollapsibleContent>
        </Collapsible>
        <VallexInputs
          vallexParams={vallexParams}
          handleChange={handleChange}
          setVallexParams={setVallexParams}
        />

        <div className="flex flex-row gap-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Vallex Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToVallex"]}
          />
          <div className="flex flex-col gap-2">
            <HyperParameters
              genParams={vallexParams}
              consumer={vallexConsumer}
              prefix="vallex"
            />
          </div>
        </div>

        <GenerationHistorySimple
          name="vallex"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey={undefined}
          filter={["sendToVallex"]}
        />
      </div>
    </Template>
  );
};

export default VallexPage;
