import React from "react";
import { AudioPlayer } from "./MemoizedWaveSurferPlayer";
import { WaveSurferOptions } from "wavesurfer.js";
import { sendToDemucs } from "../tabs/DemucsParams";
import { sendToMusicgen } from "../tabs/MusicgenParams";
import { sendToVocos } from "../tabs/VocosParams";
import { GradioFile } from "../types/GradioFile";
import { sendToBarkVoiceGeneration } from "../tabs/BarkVoiceGenerationParams";
import { sendToAceStep } from "../tabs/AceStepParams";
import { sendToKokoro } from "../tabs/KokoroParams";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { DownloadIcon, XIcon } from "lucide-react";
import { Label } from "./ui/label";
import { SingleFileUpload } from "./SingleFileUpload";

export const AudioInput = ({
  callback,
  url,
  label,
  className,
}: {
  callback: (filename?: string) => void;
  funcs?: Record<string, (audio: string | undefined | any) => void>;
  url?: string;
  label?: string;
  filter?: string[];
  metadata?: any;
  className?: string;
}) =>
  url ? (
    <div className={cn("h-36 cell flex flex-col gap-y-2 relative", className)}>
      <div className="flex items-start justify-between absolute w-full z-10 pr-4">
        <Label className="bg-background cell">{label || "Input file:"}</Label>
        {/* <Label className="">{label || "Input file:"}</Label> */}
        <Button variant="outline" size="sm" onClick={() => callback(undefined)}>
          Clear
          <XIcon className="ml-2 w-5 h-5" />
        </Button>
      </div>
      <AudioPlayerWithConfig
        // height={100}
        height="auto"
        volume={0.4}
        url={url}
      />
    </div>
  ) : (
    <SingleFileUpload
      label="Input file"
      file={url}
      accept={{ "audio/*": [".mp3", ".wav", ".flac", ".ogg", ".m4a", ".opus"] }}
      callback={(file) => callback(file)}
      className={url ? "hidden" : undefined}
    />
  );

export const AudioOutput = ({
  audioOutput,
  label,
  funcs,
  filter,
  metadata,
  className,
  ...props
}: {
  audioOutput?: GradioFile;
  label: string;
  funcs?: Record<string, (audio: string | undefined | any) => void>;
  filter?: string[];
  metadata?: any;
  className?: string;
}) => {
  return (
    <div className={cn("w-full", className)} {...props}>
      {/* {metadata && JSON.stringify(metadata)} */}
      <div className="flex items-start justify-between absolute z-10">
        <Label className="bg-background cell">{label || "Input file:"}</Label>
      </div>
      {audioOutput ? (
        <>
          <AudioPlayerWithConfig
            height={100}
            volume={0.4}
            url={audioOutput.url}
          />
          <AudioFuncs
            url={audioOutput.url}
            funcs={funcs}
            filter={filter}
            metadata={metadata}
            name={label}
          />
        </>
      ) : (
        <div className="w-full h-1">&nbsp;</div>
      )}
    </div>
  );
};

const sendToFuncs = {
  sendToDemucs,
  sendToMusicgen,
  sendToVocos,
  sendToBarkVoiceGeneration,
  sendToAceStep,
  sendToKokoro,
} as Record<string, (audio: string | undefined) => void>;

const listOfFuncs = Object.keys(sendToFuncs);

const AudioPlayerWithConfig = ({
  ...props
}: Omit<WaveSurferOptions, "container"> & {
  volume?: number;
}) => (
  <AudioPlayer
    waveColor="#ffa500"
    progressColor="#d59520"
    barWidth={4}
    barGap={1}
    barRadius={2}
    volume={0.4}
    mediaControls={true}
    {...props}
  />
);

const AudioFuncs = ({
  filter: outputFilters,
  funcs,
  url,
  metadata,
  name,
}: Omit<WaveSurferOptions, "container"> & {
  filter?: string[];
  metadata?: any;
  funcs?: Record<string, (audio: string | undefined | any) => void>;
  name?: string;
}) => (
  <div className="mt-2 flex flex-wrap gap-1">
    {funcs &&
      Object.entries(funcs).map(([funcName, func]) => (
        <FuncButton
          key={funcName}
          name={funcName}
          func={func}
          url={url}
          metadata={metadata}
        />
      ))}
    {listOfFuncs
      .filter((funcName) =>
        outputFilters ? !outputFilters.includes(funcName) : true
      )
      .map((funcName) => (
        <FuncButton
          key={funcName}
          name={funcName}
          func={sendToFuncs[funcName]}
          url={url}
          metadata={metadata}
        />
      ))}
    <DownloadButton url={url} name={name} />
  </div>
);

const DownloadButton = ({
  url,
  name = "audio",
}: {
  url?: string;
  name?: string;
}) => {
  const [downloadURL, setDownloadURL] = React.useState<string | undefined>(
    undefined
  );

  React.useEffect(() => {
    if (!url) return;
    const download = (url) => {
      if (!url) {
        throw new Error("Resource URL not provided! You need to provide one");
      }
      fetch(url)
        .then((response) => response.blob())
        .then((blob) => {
          const blobURL = URL.createObjectURL(blob);
          setDownloadURL(blobURL);
        })
        .catch((e) => console.log("=== Error downloading", e));
    };
    download(url);
  }, [url]);

  return (
    <Button variant="outline" size="sm" asChild className="flex-shrink-0">
      <a className="cursor-pointer" href={downloadURL} download={`${name}.wav`}>
        <DownloadIcon className="mr-2 w-5 h-5" />
        Download
      </a>
    </Button>
  );
};

const FuncButton = ({
  func,
  name,
  url,
  metadata,
}: {
  func: (audio: string | undefined | any, metadata?: any) => void;
  name: string;
  url: string | undefined | any;
  metadata?: any;
}) => (
  <Button variant="outline" size="sm" onClick={() => func(url, metadata)}>
    {getAudioFnName(name)}
  </Button>
);

const getAudioFnName = (name: string) =>
  name.replace(/([A-Z])/g, " $1").replace(/^./, (str) => str.toUpperCase());
