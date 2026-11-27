import React from "react";
import { Template } from "../../components/Template";
import { AudioInput, AudioOutput } from "../../components/AudioComponents";
import { useVocosResults, useVocosParams } from "../../tabs/VocosParams";
import { parseFormChange } from "../../data/parseFormChange";
import { applyVocosWav } from "../../functions/applyVocosWav";
import { VocosWavInputs } from "../../components/VocosWavInputs";
import { Button } from "../../components/ui/button";
import { toLocalCacheFile } from "../../types/LocalCacheFile";

const VocosPage = () => {
  const [vocosResult, setVocosResult] = useVocosResults();
  const [vocosParams, setVocosParams] = useVocosParams();

  async function vocos() {
    const result = await applyVocosWav(vocosParams);
    setVocosResult(result);
  }

  const useAsInput = (audio?: string) => {
    if (!audio) return;
    setVocosParams({
      ...vocosParams,
      audio: toLocalCacheFile(audio),
    });
  };

  const handleChange = parseFormChange(setVocosParams);

  return (
    <Template title="Vocos">
      <div className="flex gap-x-4 p-4">
        <div className="w-1/2 flex flex-col gap-y-2 flex-shrink-0">
          <AudioInput
            url={vocosParams?.audio?.path}
            callback={(file) => {
              setVocosParams({
                ...vocosParams,
                audio: toLocalCacheFile(file),
              });
            }}
            filter={["sendToVocos"]}
          />

          <VocosWavInputs
            vocosParams={vocosParams}
            handleChange={handleChange}
          />

          <Button variant="default" onClick={vocos}>
            Restore with Vocos
          </Button>
        </div>
        <div className="w-1/2 flex flex-col gap-y-4">
          {vocosResult && (
            <AudioOutput
              audioOutput={vocosResult}
              label="Vocos Output"
              funcs={{ useAsInput }}
              filter={["sendToVocos"]}
            />
          )}
        </div>
      </div>
    </Template>
  );
};

export default VocosPage;
