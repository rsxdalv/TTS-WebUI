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
        <ChatterboxInputs
          chatterboxParams={chatterboxParams}
          handleChange={handleChange}
          setChatterboxParams={setChatterboxParams}
          output={
            <AudioOutput
              audioOutput={historyData[0]?.audio}
              label="Generated Audio"
              funcs={funcs}
              metadata={historyData[0]}
              filter={["sendToChatterbox"]}
            />
          }
          hyperParams={
            <HyperParameters
              genParams={chatterboxParams}
              consumer={chatterboxConsumer}
              prefix="chatterbox"
            />
          }
        />

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
