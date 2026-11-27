import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { HyperParameters } from "../../components/HyperParameters";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { useAceStepPage } from "../../tabs/AceStepParams";
import { AceStepInputs } from "../../components/AceStepInputs";

const AceStepPage = () => {
  const {
    aceStepParams,
    setAceStepParams,
    historyData,
    setHistoryData,
    consumer: aceStepConsumer,
    handleChange,
    funcs,
  } = useAceStepPage();

  return (
    <Template title="ACE-Step">
      <p className="italic">
        Note: The ACE-Step extension is required to run the React UI.
      </p>
      <div className="p-4 grid grid-cols-1 gap-4">
        <AceStepInputs
          aceStepParams={aceStepParams}
          handleChange={handleChange}
          setAceStepParams={setAceStepParams}
        />
        <div className="flex gap-2 col-span-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="ACE-Step Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToAceStep"]}
          />
          <HyperParameters
            genParams={aceStepParams as any} // todo - make ace-step comply with 'text' input
            consumer={aceStepConsumer}
            prefix="aceStep"
          />
        </div>

        <GenerationHistorySimple
          name="aceStep"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="audio_duration"
          filter={["sendToAceStep"]}
        />
      </div>
    </Template>
  );
};

export default AceStepPage;
