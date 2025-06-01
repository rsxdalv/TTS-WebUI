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
import { sendToChatterbox } from "../tabs/ChatterboxParams";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import {
  SendHorizontalIcon,
  XIcon,
  Dice1Icon,
  Settings2Icon,
  HeartIcon,
  ChevronDownIcon,
  Dice5Icon,
} from "lucide-react";
import { Label } from "./ui/label";
import { SingleFileUpload } from "./SingleFileUpload";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";
import { DownloadButton } from "./DownloadButton";

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
    <div className={cn("cell flex flex-col gap-y-2 relative", className)}>
      {/* <div className="flex items-start justify-between w-full pr-4"> */}
      <div className="mb-1 flex items-start justify-between z-10">
        <Label className="bg-background/70 cell text-xs px-2 py-1">
          {label || "Input file:"}
        </Label>
        {/* <Label className="">{label || "Input file:"}</Label> */}
        <Button
          variant="ghost"
          size="sm"
          // className="absolute text-xs right-2 z-10"
          // className="z-10"
          onClick={() => callback(undefined)}
        >
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
    <div className={cn("w-full cell", className)} {...props}>
      {/* {metadata && JSON.stringify(metadata)} */}
      <div className="mb-1 flex items-start justify-between z-10">
        <Label className="bg-background/70 cell text-xs px-2 py-1">
          {label || "Input file:"}
        </Label>

        {/* Top Fns like download, favorite, copy */}
        {/* <AudioFuncs
          url={audioOutput?.url}
          funcs={funcs.filter((func) => func !== "favorite")}
          filter={filter}
          metadata={metadata}
          name={label}
        /> */}

        {/* Right side - Download button */}
        {/* <div className="flex items-center flex-1 justify-end">
          {audioOutput?.url && (

          )}
        </div> */}
        {audioOutput?.url && (
          <AudioFuncs
            url={audioOutput.url}
            funcs={funcs}
            filter={filter}
            metadata={metadata}
            name={label}
          />
        )}
      </div>
      {audioOutput ? (
        <>
          <AudioPlayerWithConfig volume={0.4} url={audioOutput.url} />
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
  sendToChatterbox,
} as Record<string, (audio: string | undefined) => void>;

const listOfFuncs = Object.keys(sendToFuncs);

const AudioPlayerWithConfig = ({
  ...props
}: Omit<WaveSurferOptions, "container"> & {
  volume?: number;
}) => (
  <AudioPlayer
    volume={props.volume || 0.4}
    height={50}
    cursorWidth={2}
    cursorColor="rgb(52, 128, 163)"
    autoplay={false}
    normalize={true}
    dragToSeek={true}
    minPxPerSec={20}
    waveColor={"rgb(129, 144, 150)"}
    // progressColor={"darkorange"}
    sampleRate={44100}
    barWidth={2}
    barGap={3}
    barRadius={10}
    // barWidth={3}
    // barGap={2}
    // barRadius={2}

    // waveColor="#ffa500"
    progressColor="rgb(78, 158, 196)"
    {...props}
  />
);
// mediaControls={true}

const AudioFuncs = ({
  filter: outputFilters,
  funcs,
  url,
  metadata,
  name,
}: Omit<WaveSurferOptions, "container"> & {
  filter?: string[];
  metadata?: any;
  funcs?: Record<
    string,
    (audio: string | undefined | any, metadata?: any) => void
  >;
  name?: string;
}) => {
  // Separate sendTo functions from other functions
  const sendToFunctions = listOfFuncs.filter((funcName) =>
    outputFilters ? !outputFilters.includes(funcName) : true
  );

  // Separate custom functions into sendTo and non-sendTo
  const customSendToFuncs = funcs
    ? Object.entries(funcs).filter(([funcName]) =>
        funcName.startsWith("sendTo")
      )
    : [];
  const customOtherFuncs = funcs
    ? Object.entries(funcs).filter(
        ([funcName]) => !funcName.startsWith("sendTo")
      )
    : [];

  return (
    <div className="justify-end flex gap-2">
      {/* Non-sendTo functions (useSeed, useParameters, favorite, etc.) */}
      {customOtherFuncs.length > 0 &&
        customOtherFuncs.map(([funcName, func]) => (
          <FuncButton
            key={funcName}
            name={funcName}
            func={func}
            url={url}
            metadata={metadata}
          />
        ))}
      {/* <DownloadButton url={audioOutput?.url} name={label} /> */}

      {/* SendTo functions in dropdown menu */}
      {(customSendToFuncs.length > 0 || sendToFunctions.length > 0) && (
        <div className="flex gap-1">
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="sm">
                <SendHorizontalIcon className="w-4 h-4 mr-2" />
                Send to...
                <ChevronDownIcon className="w-4 h-4 ml-2" />
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-56 p-1" align="start">
              <div className="flex flex-col gap-1">
                {customSendToFuncs.map(([funcName, func]) => (
                  <Button
                    key={funcName}
                    variant="ghost"
                    size="sm"
                    className="justify-start"
                    onClick={() => func(url, metadata)}
                  >
                    <SendHorizontalIcon className="w-4 h-4 mr-2" />
                    {getAudioFnName(funcName.replace("sendTo", ""))}
                  </Button>
                ))}
                {sendToFunctions.map((funcName) => (
                  <Button
                    key={funcName}
                    variant="ghost"
                    size="sm"
                    className="justify-start"
                    onClick={() => sendToFuncs[funcName](url)}
                  >
                    {getAudioFnName(funcName.replace("sendTo", ""))}
                  </Button>
                ))}
              </div>
            </PopoverContent>
          </Popover>
        </div>
      )}
      {/* <DownloadButton url={url} name={name} /> */}
    </div>
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
}) => {
  const getIconForFunction = (funcName: string) => {
    if (funcName === "useSeed") return <Dice5Icon className="w-4 h-4" />;
    if (funcName === "useParameters")
      return <Settings2Icon className="w-4 h-4" />;
    if (funcName === "favorite") return <HeartIcon className="w-4 h-4" />;
    // Use As History
    // Use As History Prompt Semantic
    if (funcName === "useAsHistory")
      return <SendHorizontalIcon className="w-4 h-4" />;
    if (funcName === "useAsHistoryPromptSemantic")
      return <SendHorizontalIcon className="w-4 h-4" />;
    return null;
  };

  const icon = getIconForFunction(name);
  const displayName = getAudioFnName(name);

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => func(url, metadata)}
    >
      {icon ? (
        <span className="flex items-center gap-2">
          {icon}
          {displayName}
        </span>
      ) : (
        displayName
      )}
    </Button>
  );
};

const getAudioFnName = (name: string) =>
  name.replace(/([A-Z])/g, " $1").replace(/^./, (str) => str.toUpperCase());

