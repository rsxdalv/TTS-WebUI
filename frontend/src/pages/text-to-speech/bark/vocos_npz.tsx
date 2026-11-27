import React from "react";
import { Template } from "../../../components/Template";
import useLocalStorage from "../../../hooks/useLocalStorage";
import { AudioOutput } from "../../../components/AudioComponents";
import {
  useVocosParamsNPZ,
  useVocosResultsNPZ,
} from "../../../tabs/VocosParamsNPZ";
import { GradioFile } from "../../../types/GradioFile";
import { encodecDecode } from "../../../functions/encodecDecode";
import { applyVocosNPZ } from "../../../functions/applyVocosNPZ";
import { Button } from "../../../components/ui/button";
import { SingleFileUpload } from "../../../components/SingleFileUpload";
import { toLocalCacheFile } from "../../../types/LocalCacheFile";

const VocosPageNPZ = () => {
  const [dataEncodec, setDataEncodec] = useLocalStorage<GradioFile | null>(
    "vocosOutputNpzEncodec",
    null
  );
  const [vocosResult, setVocosResult] = useVocosResultsNPZ();
  const [vocosParamsNPZ, setVocosParamsNPZ] = useVocosParamsNPZ();

  const vocos = async () => setVocosResult(await applyVocosNPZ(vocosParamsNPZ));
  const decodeWithEncodec = async () =>
    setDataEncodec(await encodecDecode(vocosParamsNPZ));

  return (
    <Template title="Vocos NPZ">
      <div className="flex gap-x-4 p-4">
        <div className="flex flex-col gap-y-2 w-1/2">
          <SingleFileUpload
            label="Input NPZ"
            file={vocosParamsNPZ?.npz_file?.path}
            accept={{ "application/x-npz": [".npz"] }}
            callback={(npz_file) => {
              setVocosParamsNPZ({
                ...vocosParamsNPZ,
                npz_file: toLocalCacheFile(npz_file),
              });
            }}
          />

          <div className="grid grid-cols-2 gap-x-2">
            <Button variant="default" onClick={decodeWithEncodec}>
              Decode with Encodec
            </Button>
            <Button variant="default" onClick={vocos}>
              Restore with Vocos
            </Button>
          </div>
        </div>
        <div className="flex flex-col gap-y-4">
          {vocosResult && (
            <AudioOutput audioOutput={vocosResult} label="Vocos Output" />
          )}
          {dataEncodec && (
            <AudioOutput audioOutput={dataEncodec} label="Encodec Output" />
          )}
        </div>
      </div>
    </Template>
  );
};

export default VocosPageNPZ;
