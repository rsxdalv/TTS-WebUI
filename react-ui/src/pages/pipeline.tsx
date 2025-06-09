import React from "react";
import { Template } from "../components/Template";
import useLocalStorage from "../hooks/useLocalStorage";
import { AudioOutput } from "../components/AudioComponents";
import { GradioFile } from "../types/GradioFile";
import { splitWithDemucs } from "../functions/splitWithDemucs";
import { generateWithBark } from "../functions/generateWithBark";
import { BarkInputs } from "../components/BarkInputs";
import {
  getBarkGenerationParams,
  useBarkGenerationParams,
} from "../tabs/BarkGenerationParams";
import { RVCInputs } from "../components/RVCInputs";
import {
  getRVCGenerationParams,
  useRVCGenerationParams,
} from "../tabs/RVCParams";
import { applyRVC } from "../functions/applyRVC";
import { TortoiseInput } from "../components/TortoiseInput";
import { generateWithTortoise } from "../functions/generateWithTortoise";
import {
  getTortoiseGenerationParams,
  TortoiseGenerationParams,
  useTortoiseGenerationParams,
} from "../tabs/TortoiseGenerationParams";
import { parseFormChange } from "../data/parseFormChange";
import { getMagnetParams, useMagnetParams } from "../tabs/MagnetParams";
import { generateWithMagnet } from "../functions/generateWithMagnet";
import { MagnetInputs } from "../components/MagnetInputs";
import { getMusicgenParams, useMusicgenParams } from "../tabs/MusicgenParams";
import { generateWithMusicgen } from "../functions/generateWithMusicgen";
import { MusicgenInputs } from "../components/MusicgenInputs";
import { VocosWavInputs } from "../components/VocosWavInputs";
import { getVocosParams, useVocosParams } from "../tabs/VocosParams";
import { applyVocosWav } from "../functions/applyVocosWav";
import { applyVocosNPZ } from "../functions/applyVocosNPZ";
import { getWebuiURLWithHost } from "../data/getWebuiURL";
import {
  getMahaParams,
  initialMahaParams,
  useMahaParams,
} from "../tabs/MahaParams";
import { generateWithMaha } from "../functions/generateWithMaha";
import { MahaInputs } from "../components/MahaInputs";
import { getMMSParams, useMMSParams } from "../tabs/MMSParams";
import { generateWithMMS } from "../functions/generateWithMMS";
import { MMSInputs } from "../components/MMSInputs";
import { getVallexParams, useVallexParams } from "../tabs/VallexParams";
import { generateWithVallex } from "../functions/generateWithVallex";
import { VallexInputs } from "../components/VallexInputs";
import { applySeed } from "../data/applySeed";
import { Button } from "../components/ui/button";
import { RadioWithLabel } from "../components/component/RadioWithLabel";
import { toLocalCacheFile } from "../types/LocalCacheFile";

interface PipelineParams {
  generation: string;
  postprocess: string;
}

const initialState: PipelineParams = {
  generation: "bark",
  postprocess: "demucs",
};

const pipelineId = "pipeline";

async function getResult(model: string) {
  const paramsMap = {
    bark: getBarkGenerationParams,
    tortoise: getTortoiseGenerationParams,
    musicgen: getMusicgenParams,
    magnet: getMagnetParams,
    maha: getMahaParams,
    mms: getMMSParams,
    vallex: getVallexParams,
  };
  const newParams = applySeed(paramsMap[model as keyof typeof paramsMap]());

  const fns = {
    bark: generateWithBark,
    tortoise: (x: TortoiseGenerationParams) =>
      generateWithTortoise({
        ...x,
        candidates: 1,
      })[0],
    musicgen: generateWithMusicgen,
    magnet: generateWithMagnet,
    maha: generateWithMaha,
    mms: generateWithMMS,
    vallex: generateWithVallex,
  };

  return {
    result: await fns[model](newParams),
    newSeed: newParams.use_random_seed ? newParams.seed : undefined,
  };
}

const GenerateButton = ({
  status,
  onClick,
}: {
  status: string;
  onClick: () => void;
}) => (
  <Button className="cell font-medium" onClick={onClick}>
    {status === "generating" && "Generating..."}
    {status === "postprocessing" && "Postprocessing..."}
    {status === "idle" && "Run Pipeline"}
  </Button>
);

const GenerationInputs = ({ model, seed }: { model: string; seed: number }) => {
  const [barkGenerationParams, setBarkVoiceGenerationParams] =
    useBarkGenerationParams();
  const [tortoiseGenerationParams, setTortoiseGenerationParams] =
    useTortoiseGenerationParams();
  const [musicgenParams, setMusicgenParams] = useMusicgenParams();
  const [magnetParams, setMagnetParams] = useMagnetParams();
  const [mahaParams, setMahaParams] = useMahaParams();
  const [mmsParams, setMmsParams] = useMMSParams();
  const [vallexParams, setVallexParams] = useVallexParams();

  React.useEffect(() => {
    const fn = {
      bark: setBarkVoiceGenerationParams,
      tortoise: setTortoiseGenerationParams,
      musicgen: setMusicgenParams,
      magnet: setMagnetParams,
      maha: setMahaParams,
      mms: setMmsParams,
      vallex: setVallexParams,
    };
    fn[model as keyof typeof fn]((x) => ({ ...x, seed }));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seed, model]);

  switch (model) {
    case "bark":
      return (
        <BarkInputs
          barkGenerationParams={barkGenerationParams}
          handleChange={parseFormChange(setBarkVoiceGenerationParams)}
        />
      );
    case "tortoise":
      return (
        <TortoiseInput
          tortoiseGenerationParams={tortoiseGenerationParams}
          setTortoiseGenerationParams={setTortoiseGenerationParams}
          handleChange={parseFormChange(setTortoiseGenerationParams)}
        />
      );
    case "musicgen":
      return (
        <MusicgenInputs
          musicgenParams={musicgenParams}
          handleChange={parseFormChange(setMusicgenParams)}
          setMusicgenParams={setMusicgenParams}
        />
      );
    case "magnet":
      return (
        <MagnetInputs
          magnetParams={magnetParams}
          handleChange={parseFormChange(setMagnetParams)}
        />
      );
    case "maha":
      return (
        <MahaInputs
          mahaParams={mahaParams}
          handleChange={parseFormChange(setMahaParams)}
          resetParams={() =>
            setMahaParams({ ...mahaParams, ...initialMahaParams })
          }
        />
      );
    case "mms":
      return (
        <MMSInputs
          mmsParams={mmsParams}
          handleChange={parseFormChange(setMmsParams)}
          setMmsParams={setMmsParams}
        />
      );
    case "vallex":
      return (
        <VallexInputs
          vallexParams={vallexParams}
          handleChange={parseFormChange(setVallexParams)}
          setVallexParams={setVallexParams}
        />
      );
    default:
      return null;
  }
};

async function postProcessAudio(
  model: string,
  { url }: GradioFile,
  npz_file?: string
): Promise<GradioFile[]> {
  switch (model) {
    case "demucs":
      return splitWithDemucs({ audio: toLocalCacheFile(url) });
    case "rvc":
      return [
        (
          await applyRVC({
            ...getRVCGenerationParams(),
            original_audio: url,
          })
        ).audio,
      ];
    case "vocos wav":
      return [
        await applyVocosWav({
          ...getVocosParams(),
          audio: toLocalCacheFile(url),
        }),
      ];
    case "vocos npz (bark only)":
      if (!npz_file) return [];
      return [
        await applyVocosNPZ({
          npz_file: toLocalCacheFile(getWebuiURLWithHost(npz_file)),
        }),
      ];
    case "none":
    default:
      return [];
  }
}

const PipelinePage = () => {
  const [output, setOutput] = useLocalStorage<GradioFile[] | null>(
    "pipelineOutput",
    null
  );
  const [pipelineParams, setPipelineParams] = useLocalStorage<PipelineParams>(
    pipelineId,
    initialState
  );

  const [status, setStatus] = React.useState("idle");

  const [seed, setSeed] = React.useState<number>(0);

  async function pipeline() {
    const { generation, postprocess } = pipelineParams;
    setStatus("generating");
    const { result, newSeed } = await getResult(generation);
    if (newSeed) {
      setSeed(newSeed);
    }
    if (!result) return;
    const { audio } = result;
    setOutput([audio]);
    setStatus("postprocessing");
    const postProcessed = await postProcessAudio(
      postprocess,
      audio,
      result?.npz
    );
    setOutput([audio, ...postProcessed]);
    setStatus("idle");
  }

  const generationModels = [
    "bark",
    "tortoise",
    "musicgen",
    "magnet",
    "maha",
    "mms",
    "vallex",
  ];

  const onChangeModel = (event: React.ChangeEvent<HTMLInputElement>) => {
    const model = event.target.value;
    if (
      model !== "bark" &&
      pipelineParams.postprocess === "vocos npz (bark only)"
    ) {
      setPipelineParams((x) => ({ ...x, postprocess: "none" }));
    }
    setPipelineParams((x) => ({ ...x, generation: model }));
  };

  const PostProcessInputs = ({ model }: { model: string }) => {
    const [rvcGenerationParams, setRvcGenerationParams] =
      useRVCGenerationParams();
    const [vocosParams, setVocosParams] = useVocosParams();

    switch (model) {
      case "rvc":
        return (
          <RVCInputs
            rvcParams={rvcGenerationParams}
            handleChange={parseFormChange(setRvcGenerationParams)}
            hideAudioInput
          />
        );
      case "vocos wav":
        return (
          <VocosWavInputs
            vocosParams={vocosParams}
            handleChange={parseFormChange(setVocosParams)}
          />
        );
      default:
        return null;
    }
  };

  const postProcessingModels = [
    "none",
    "rvc",
    "demucs",
    "vocos wav",
    "vocos npz (bark only)",
  ];

  return (
    <Template title="Pipeline">
      <div className="flex flex-col gap-y-4 p-4">
        <div className="flex flex-col gap-y-2">
          <p className="font-semibold">
            The pipeline is still a work in progress and might change in future
            updates.
          </p>
          <p>
            This pipeline takes an audio file as input and runs it through a
            generation model to generate a representation of the audio. This
            representation is then refined by a postprocess model to generate a
            new audio file.
          </p>
          {/* <div className="flex flex-row gap-x-2 cell mb-4">
            <label>Choose a generation model:</label>
            {generationModels.map((model) => (
              <div key={model} className="flex items-center gap-x-2">
                <input
                  type="radio"
                  name="generation"
                  id={model}
                  value={model}
                  checked={pipelineParams.generation === model}
                  onChange={onChangeModel}
                />
                <label htmlFor={model}>{model}</label>
              </div>
            ))}
          </div> */}
          <RadioWithLabel
            label="Choose a generation model"
            name="generation"
            className="cell"
            inline
            value={pipelineParams.generation}
            onChange={onChangeModel}
            options={generationModels.map((model) => ({
              label: model,
              value: model,
            }))}
          />
          <GenerationInputs model={pipelineParams.generation} seed={seed} />
          <RadioWithLabel
            label="Choose a postprocessing model"
            name="postprocess"
            className="cell"
            inline
            value={pipelineParams.postprocess}
            onChange={(event) =>
              setPipelineParams((x) => ({
                ...x,
                postprocess: event.target.value,
              }))
            }
            options={postProcessingModels.map((model) => ({
              label: model,
              value: model,
              disabled:
                model === "vocos npz (bark only)" &&
                pipelineParams.generation !== "bark",
            }))}
          />
          <PostProcessInputs model={pipelineParams.postprocess} />
          <GenerateButton status={status} onClick={pipeline} />
        </div>
        <div className="flex flex-col gap-y-4">
          {output &&
            output.map((item, index) => (
              <AudioOutput
                key={index}
                audioOutput={item}
                label={
                  index === 0
                    ? "Generated Audio"
                    : `Post-processed ${index}/${output.length - 1}`
                }
                funcs={{}}
                metadata={item}
                filter={["sendToPipeline"]}
              />
            ))}
        </div>
      </div>
    </Template>
  );
};

export default PipelinePage;
