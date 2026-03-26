import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { HyperParameters } from "../../components/HyperParameters";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { MiniMaxTTSInputs } from "../../components/MiniMaxTTSInputs";
import { useMiniMaxTTSPage } from "../../tabs/MiniMaxTTSParams";

const MiniMaxTTSPage = () => {
  const {
    historyData,
    setHistoryData,
    minimaxTTSParams,
    setMiniMaxTTSParams,
    consumer: minimaxTTSConsumer,
    handleChange,
    funcs,
  } = useMiniMaxTTSPage();

  return (
    <Template title="MiniMax Cloud TTS">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <MiniMaxTTSInputs
          minimaxTTSParams={minimaxTTSParams}
          handleChange={handleChange}
          output={
            <AudioOutput
              audioOutput={historyData[0]?.audio}
              label="MiniMax Cloud TTS Output"
              funcs={funcs}
              metadata={historyData[0]}
              filter={["sendToMiniMaxTTS"]}
            />
          }
          hyperParams={
            <HyperParameters
              genParams={minimaxTTSParams}
              consumer={minimaxTTSConsumer}
              prefix="minimax_cloud_tts"
            />
          }
        />

        <GenerationHistorySimple
          name="MiniMax Cloud TTS"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey={undefined}
          filter={["sendToMiniMaxTTS"]}
        />
      </div>
    </Template>
  );
};

export default MiniMaxTTSPage;
