import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { useMahaPage } from "../../tabs/MahaParams";
import { HyperParameters } from "../../components/HyperParameters";
import { MahaInputs } from "../../components/MahaInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";

const MahaPage = () => {
  const {
    historyData,
    setHistoryData,
    mahaParams,
    resetParams,
    consumer: mahaConsumer,
    handleChange,
    funcs,
  } = useMahaPage();

  return (
    <Template title="Maha">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <MahaInputs
          mahaParams={mahaParams}
          handleChange={handleChange}
          resetParams={resetParams}
        />

        <div className="flex flex-row gap-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Maha Output"
            funcs={funcs}
            metadata={historyData[0]}
          />
          <div className="flex flex-col gap-2">
            <HyperParameters
              genParams={mahaParams}
              consumer={mahaConsumer}
              prefix="maha"
            />
          </div>
        </div>

        <GenerationHistorySimple
          name="maha"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey={undefined}
          filter={["sendToMaha"]}
        />
      </div>
    </Template>
  );
};

export default MahaPage;
