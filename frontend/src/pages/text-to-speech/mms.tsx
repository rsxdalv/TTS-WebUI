import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { useMMSPage } from "../../tabs/MMSParams";
import { HyperParameters } from "../../components/HyperParameters";
import { MMSInputs } from "../../components/MMSInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";

const MMSPage = () => {
  const {
    historyData,
    setHistoryData,
    mmsParams,
    setMMSParams,
    consumer: mmsConsumer,
    handleChange,
    funcs,
  } = useMMSPage();

  return (
    <Template title="MMS">
      <div className="gap-y-4 p-4 flex w-full flex-col">
        <MMSInputs
          mmsParams={mmsParams}
          handleChange={handleChange}
          setMmsParams={setMMSParams}
        />

        <div className="flex flex-row gap-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="MMS Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToMMS"]}
          />
          <HyperParameters
            genParams={mmsParams}
            consumer={mmsConsumer}
            prefix="mms"
          />
        </div>

        <GenerationHistorySimple
          name="mms"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey={undefined}
          filter={["sendToMMS"]}
        />
      </div>
    </Template>
  );
};

export default MMSPage;
