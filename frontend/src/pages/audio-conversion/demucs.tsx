import React from "react";
import { Template } from "../../components/Template";
import { AudioInput, AudioOutput } from "../../components/AudioComponents";
import { useDemucsPage } from "../../tabs/DemucsParams";
import { GenerationHistorySimple } from "../../components/GenerationHistory";
import { Button } from "../../components/ui/button";
import { toLocalCacheFile } from "../../types/LocalCacheFile";

const DemucsPage = () => {
  const {
    demucsParams,
    setDemucsParams,
    historyData,
    setHistoryData,
    consumer: demucs,
    funcs,
  } = useDemucsPage();

  return (
    <Template title="Demucs">
      <div className="flex flex-col gap-4">
        <div className="flex gap-4 w-full">
          <div className="flex flex-col gap-y-2 w-96">
            <AudioInput
              url={demucsParams?.audio?.path}
              callback={(file) => {
                setDemucsParams({
                  ...demucsParams,
                  // audio: file,
                  audio: toLocalCacheFile(file),
                });
              }}
              filter={["sendToDemucs"]}
            />

            <Button onClick={() => demucs(demucsParams)}>
              Split with Demucs
            </Button>
          </div>
          <div className="flex flex-col gap-y-4">
            {historyData.slice(0, 4).map(({ audio, typeName }) => (
              <AudioOutput
                key={typeName}
                audioOutput={audio}
                label={typeName}
                funcs={funcs}
                filter={["sendToDemucs"]}
              />
            ))}
          </div>
        </div>

        <GenerationHistorySimple
          name="demucs"
          setHistoryData={setHistoryData}
          historyData={historyData.slice(3)}
          funcs={funcs}
          nameKey="typeName"
          filter={["sendToDemucs"]}
        />
      </div>
    </Template>
  );
};

export default DemucsPage;
