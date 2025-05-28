import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { KokoroInputs } from "../../components/KokoroInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { HyperParameters } from "../../components/HyperParameters";
import { useKokoroPage } from "../../tabs/KokoroParams";

const KokoroPage = () => {
  const {
    historyData,
    setHistoryData,
    kokoroParams,
    setKokoroParams,
    resetParams,
    consumer: kokoroGeneration,
    funcs,
    handleChange,
  } = useKokoroPage();

  return (
    <Template title="Kokoro TTS">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <div className="flex gap-x-6 w-full justify-center">
          <div className="flex flex-col gap-y-4 w-1/2">
            <KokoroInputs
              kokoroParams={kokoroParams}
              handleChange={handleChange}
              setKokoroParams={setKokoroParams}
              resetParams={resetParams}
            />
          </div>
          <div className="flex flex-col gap-y-4 w-1/2">
            <AudioOutput
              audioOutput={historyData[0]?.audio}
              label="Kokoro TTS Output"
              funcs={funcs}
              metadata={historyData[0]}
              filter={["sendToKokoro"]}
            />
            <HyperParameters
              genParams={kokoroParams}
              // onChange={handleChange}
              consumer={kokoroGeneration}
              prefix="kokoro"
            />
          </div>
        </div>
        <GenerationHistorySimple
          name="Kokoro TTS"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="text"
          filter={["sendToKokoro"]}
        />
      </div>
    </Template>
  );
};

export default KokoroPage;
