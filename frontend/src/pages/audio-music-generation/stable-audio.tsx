import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { HyperParameters } from "../../components/HyperParameters";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { useStableAudioPage } from "../../tabs/StableAudioParams";
import { StableAudioInputs } from "../../components/StableAudioInputs";

const StableAudioPage = () => {
  const {
    stableAudioParams,
    setStableAudioParams,
    historyData,
    setHistoryData,
    consumer: stableAudioConsumer,
    handleChange,
    funcs,
  } = useStableAudioPage();

  return (
    <Template title="Stable Audio">
      <div className="p-4 grid grid-cols-1 gap-4">
        <StableAudioInputs
          stableAudioParams={stableAudioParams}
          handleChange={handleChange}
          setStableAudioParams={setStableAudioParams}
        />
        <div className="flex gap-2 col-span-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Stable Audio Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToStableAudio"]}
          />
          <HyperParameters
            genParams={stableAudioParams}
            consumer={stableAudioConsumer}
            prefix="stableAudio"
          />
        </div>

        <GenerationHistorySimple
          name="stableAudio"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="folder_root"
          filter={["sendToStableAudio"]}
        />
      </div>
    </Template>
  );
};

export default StableAudioPage;
