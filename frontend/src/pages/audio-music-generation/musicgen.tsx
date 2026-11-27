import React from "react";
import { Template } from "../../components/Template";
import { AudioOutput } from "../../components/AudioComponents";
import { useMusicgenPage } from "../../tabs/MusicgenParams";
import { HyperParameters } from "../../components/HyperParameters";
import { MusicgenInputs } from "../../components/MusicgenInputs";
import { GenerationHistorySimple } from "../../components/GenerationHistory";

const MusicgenPage = () => {
  const {
    musicgenParams,
    setMusicgenParams,
    historyData,
    setHistoryData,
    consumer: musicgenConsumer,
    handleChange,
    funcs,
  } = useMusicgenPage();

  return (
    <Template title="Musicgen">
      <div className="p-4 grid grid-cols-1 gap-4">
        <MusicgenInputs
          musicgenParams={musicgenParams}
          handleChange={handleChange}
          setMusicgenParams={setMusicgenParams}
        />
        <div className="flex gap-2 col-span-2">
          <AudioOutput
            audioOutput={historyData[0]?.audio}
            label="Musicgen Output"
            funcs={funcs}
            metadata={historyData[0]}
            filter={["sendToMusicgen"]}
          />
          <HyperParameters
            genParams={musicgenParams}
            consumer={musicgenConsumer}
            prefix="musicgen"
          />
        </div>

        <GenerationHistorySimple
          name="musicgen"
          setHistoryData={setHistoryData}
          historyData={historyData}
          funcs={funcs}
          nameKey="folder_root"
          filter={["sendToMusicgen"]}
        />
      </div>
    </Template>
  );
};

export default MusicgenPage;
