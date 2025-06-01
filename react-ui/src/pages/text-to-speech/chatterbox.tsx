import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { HyperParameters } from "../../components/HyperParameters";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { ChatterboxInputs } from "../../components/ChatterboxInputs";
import { useChatterboxPage } from "../../tabs/ChatterboxParams";

const ChatterboxPage = () => {
  const {
    historyData,
    setHistoryData,
    chatterboxParams,
    setChatterboxParams,
    consumer: chatterboxConsumer,
    handleChange,
    funcs,
  } = useChatterboxPage();

  return (
    <Template title="Chatterbox TTS">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <div className="flex gap-x-6 w-full justify-center">
          <div className="flex flex-col gap-y-4 w-1/2">
            <ChatterboxInputs
              chatterboxParams={chatterboxParams}
              handleChange={handleChange}
              setChatterboxParams={setChatterboxParams}
            />
          </div>
          <div className="flex flex-col gap-y-4 w-1/2">
            <AudioOutput
              audioOutput={historyData[0]?.audio}
              label="Chatterbox TTS Output"
              funcs={funcs}
              metadata={historyData[0]}
              filter={["sendToChatterbox"]}
            />
            <HyperParameters
              genParams={chatterboxParams}
              consumer={chatterboxConsumer}
              prefix="chatterbox"
            />
          </div>
        </div>

        <GenerationHistorySimple
          name="Chatterbox TTS"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey={undefined}
          filter={["sendToChatterbox"]}
        />
      </div>
    </Template>
  );
};

export default ChatterboxPage;