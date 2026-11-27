import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { TortoiseInput } from "../../components/TortoiseInput";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { HyperParameters } from "../../components/HyperParameters";
import { useTortoisePage } from "../../tabs/TortoiseGenerationParams";

const TortoiseGenerationPage = () => {
  const {
    historyData,
    setHistoryData,
    tortoiseGenerationParams,
    setTortoiseGenerationParams,
    consumer: tortoiseGeneration,
    funcs,
    handleChange,
  } = useTortoisePage();

  return (
    <Template title="Tortoise">
      <div className="gap-y-4 pt-4 flex w-full flex-col">
        <TortoiseInput
          tortoiseGenerationParams={tortoiseGenerationParams}
          setTortoiseGenerationParams={setTortoiseGenerationParams}
          handleChange={handleChange}
        />
        <div className="flex flex-row gap-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Tortoise Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToTortoise", "sendToTortoiseVoiceGeneration"]}
          />
          <HyperParameters
            genParams={tortoiseGenerationParams}
            consumer={tortoiseGeneration}
            prefix="tortoise"
          />
        </div>

        <GenerationHistorySimple
          name="tortoise"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="bundle_name"
          filter={["sendToTortoise", "sendToTortoiseVoiceGeneration"]}
        />
      </div>
    </Template>
  );
};

export default TortoiseGenerationPage;
