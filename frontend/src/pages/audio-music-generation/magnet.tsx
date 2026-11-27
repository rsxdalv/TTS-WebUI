import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { useMagnetPage } from "../../tabs/MagnetParams";
import { HyperParameters } from "../../components/HyperParameters";
import { parseFormChange } from "../../data/parseFormChange";
import { MagnetInputs } from "../../components/MagnetInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";

const MagnetPage = () => {
  const {
    historyData,
    setHistoryData,
    magnetParams,
    setMagnetParams,
    consumer,
    funcs,
  } = useMagnetPage();

  return (
    <Template title="Magnet">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <MagnetInputs
          magnetParams={magnetParams}
          handleChange={parseFormChange(setMagnetParams)}
        />

        <div className="flex flex-row gap-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Magnet Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToMagnet"]}
          />
          <HyperParameters
            genParams={magnetParams}
            consumer={consumer}
            prefix="magnet"
          />
        </div>

        <GenerationHistorySimple
          name="magnet"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="folder_root"
          filter={["sendToMagnet"]}
        />
      </div>
    </Template>
  );
};

export default MagnetPage;
