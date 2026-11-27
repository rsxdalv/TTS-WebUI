import AuthorIcon from "@material-design-icons/svg/filled/person.svg";
import DownloadIcon from "@material-design-icons/svg/filled/download.svg";
import WomanIcon from "@material-design-icons/svg/filled/female.svg";
import ManIcon from "@material-design-icons/svg/filled/male.svg";
import OtherIcon from "@material-design-icons/svg/filled/alt_route.svg";
import PlayIcon from "@material-design-icons/svg/filled/play_arrow.svg";
import PauseIcon from "@material-design-icons/svg/filled/pause.svg";
import AddIcon from "@material-design-icons/svg/filled/add.svg";
import RecordVoiceOverIcon from "@material-design-icons/svg/filled/record_voice_over.svg";
import OpenFolderIcon from "@material-design-icons/svg/filled/folder_open.svg";
import PlaylistAddIcon from "@material-design-icons/svg/filled/playlist_add.svg";
import StarIcon from "@material-design-icons/svg/filled/star.svg";
import StarIconOutlined from "@material-design-icons/svg/outlined/star.svg";
import DeleteForeverIcon from "@material-design-icons/svg/filled/delete_forever.svg";
import React, { useEffect, useRef, useState } from "react";
import { Flag } from "./Flag";
import { Voice } from "../types/Voice";
import { useFavorites, saveOrDeleteFromFavorites } from "./FavoritesProvider";
import { useLocalVotes, useVotes } from "./VotesProvider";
import { Vote } from "./Vote";
import { MUIIcon } from "./mini/MUIIcon";
import { GenerationRaw } from "../types/Generation";
import { parseMetadataDate } from "./parseMetadataDate";
import { Metadata, Row } from "./Metadata";
import { sendToBarkAsVoice } from "../tabs/BarkGenerationParams";
import { NPZ, NPZOptional } from "../types/NPZ";
import { favorite } from "../functions/favorite";
import { saveToVoices } from "../functions/saveToVoices";
import { PlayCircleIcon, SendHorizontalIcon, SendIcon } from "lucide-react";
import { Button } from "./ui/button";
import { getWebuiURL, getWebuiURLWithHost } from "../data/getWebuiURL";
import { encodecDecode } from "../functions/encodecDecode";
import { toLocalCacheFile } from "../types/LocalCacheFile";

const ActionButton = ({
  icon,
  alt,
  onClick,
}: {
  icon: { src: string; width: number; height: number };
  alt: string;
  onClick: () => void;
}) => (
  <button
    className="w-9 h-9 text-xl text-gray-900 select-none hover:opacity-70"
    onClick={onClick}
    title={alt}
  >
    <MUIIcon icon={icon} alt={alt} className="w-8 h-8 rounded" />
  </button>
);

export const CardBig = ({
  voice: { name, audio, download, image, tags, language, author, gender },
}: {
  voice: Voice;
}) => {
  const [votes, setVote] = useVotes();
  const voteCount = votes[download] || 0;
  const [localVotes, setLocalVotes] = useLocalVotes();
  const vote = localVotes[download] || 0;
  const setVoteWithLocalStorage = (newVote: number) => {
    setVote(download, newVote - vote);
    setLocalVotes({ ...localVotes, [download]: newVote });
  };
  return (
    <div className="flex flex-col items-center justify-start w-full max-w-md py-4 px-6 bg-white rounded shadow-lg">
      <div className="flex flex-col gap-y-4 w-full h-full justify-between">
        <div className="flex items-center w-full gap-x-2">
          <h1 className="text-2xl font-bold text-gray-900">{name}</h1>
          <div className="ml-auto" />
          <Author author={author} />
          <SaveToFavorites download={download} />
          <Gender gender={gender} />
          <Flag language={language} />
        </div>
        <div className="flex w-full gap-x-4">
          <img
            className="w-24 h-24 rounded select-none"
            src={image}
            width={96}
            height={96}
            alt="voice"
          />
          <Tags tags={tags} />
        </div>
        <div className="flex w-full justify-between">
          <AudioPlayer audio={audio} />
          <Vote
            vote={vote}
            setVote={setVoteWithLocalStorage}
            voteCount={voteCount}
          />
          <Download download={download} />
        </div>
      </div>
    </div>
  );
};

export const CardGeneration = ({
  metadata: {
    prompt,
    _type,
    text,
    language,
    history_hash,
    filename,
    date,
    ...rest
  },
}: {
  metadata: GenerationRaw;
}) => {
  const promptText = prompt || text || "";
  // Detect if prompt is Japanese
  const isJapanese = promptText.match(/[\u3040-\u309F\u30A0-\u30FF]/);
  const maxLength = isJapanese ? 30 : 50;
  // const maxLength = 100000;
  return (
    <div className="flex flex-col items-center justify-start w-full max-w-md py-4 px-6 bg-white rounded shadow-lg">
      <div className="flex flex-col gap-y-4 w-full h-full justify-between">
        <div className="flex w-full">
          <h1 className="text-2xl font-bold text-gray-900">
            <span
              className={
                promptText.length > maxLength
                  ? "text-xl font-bold text-gray-900"
                  : "text-2xl font-bold text-gray-900"
              }
            >
              {promptText.length > maxLength
                ? promptText.substring(0, maxLength) + "..."
                : promptText}
            </span>
          </h1>
        </div>
        {/* <p className="text-gray-500">{filename}</p> */}
        <div className="flex w-full justify-between">
          <AudioPlayer audio={filename} />
          <p className="text-gray-500">{prettifyDate(date)}</p>
          {/* {language && <Flag language={parseMetadataLanguage(language)} />} */}
        </div>
        {/* <a
          href={`#voices/${history_hash}`}
          target="_blank"
          className="text-blue-500 hover:underline"
        >
          Voice
        </a> */}
        {/* <a
          href={`#generations/${history_hash}`}
          target="_blank"
          className="text-blue-500 hover:underline"
        >
          Generation info
        </a> */}
        <div>
          <div className="flex flex-col">
            <Row label="Type" value={_type} />
          </div>
        </div>
        {_type === "tortoise" ? (
          <div className="text-xs text-gray-500 flex flex-col w-full break-words">
            {/* render everything as rows */}
            <div className="flex flex-col">
              <Row label="Prompt" value={prompt} />
              <Row label="Language" value={language} />
              <Row label="History Hash" value={history_hash} />
              {Object.entries(rest).map(([key, value]) => (
                <Row key={key} label={key} value={value} />
              ))}
            </div>
          </div>
        ) : (
          <Metadata
            prompt={prompt}
            language={language || "unknown"}
            history_hash={history_hash}
            {...rest}
          />
        )}
      </div>
    </div>
  );
};

export const HistoryCard = ({
  metadata: {
    prompt,
    _type,
    text,
    language,
    history_hash,
    filename,
    date,
    folder_root,
    api_filename,
    ...rest
  },
  isFavorite,
}: {
  metadata: GenerationRaw;
  isFavorite?: boolean;
}) => {
  const promptText = prompt || text || "(No title)";
  // Detect if prompt is Japanese
  const isJapanese = promptText.match(/[\u3040-\u309F\u30A0-\u30FF]/);
  const maxLength = isJapanese ? 30 : 50;
  // const maxLength = 100000;
  const absoluteFilename = "/" + filename;

  const deleteFavorite = async (
    _url: string,
    data?: {
      folder_root?: string;
    }
  ) => {
    const folder_root = data?.folder_root;
    if (!folder_root) return;
    const response = await fetch("/api/gradio/delete_generation", {
      method: "POST",
      body: JSON.stringify({
        folder_root,
      }),
    });
    return await response.json();
  };

  const addFavorite = () => favorite("", { folder_root });
  const removeFavorite = () => deleteFavorite("", { folder_root });
  const openFolder = () => {
    fetch("/api/gradio/open_folder", {
      method: "POST",
      body: JSON.stringify({ folder_path: folder_root }),
    });
  };
  const useAsVoice = () => {
    const history_npz = api_filename?.replace(".ogg", ".npz");
    sendToBarkAsVoice(history_npz);
  };

  const ActionRow = ({
    isFavorite,
    removeFavorite,
    addFavorite,
    saveToVoices,
    openFolder,
    useAsVoice,
    filename,
    _type,
  }: {
    isFavorite?: boolean;
    removeFavorite: () => void;
    addFavorite: () => void;
    saveToVoices: (api_filename?: string) => void;
    openFolder: () => void;
    useAsVoice: () => void;
    filename: string;
    _type?: string;
  }) => {
    return (
      <div className="flex w-full justify-between">
        {isFavorite ? (
          // <ActionButton
          //   icon={StarIcon}
          //   alt="Remove from favorites"
          //   onClick={removeFavorite}
          //   />
          <ActionButton
            icon={DeleteForeverIcon}
            alt="Delete from favorites"
            onClick={removeFavorite}
          />
        ) : (
          <>
            <ActionButton
              icon={DeleteForeverIcon}
              alt="Delete"
              onClick={removeFavorite}
            />
            <ActionButton
              icon={StarIconOutlined}
              alt="Add to favorites"
              onClick={addFavorite}
            />
          </>
        )}
        <Download download={filename} />
        {_type === "bark" && (
          <ActionButton
            icon={PlaylistAddIcon}
            alt="Save to voices"
            onClick={() => saveToVoices(api_filename)}
          />
        )}
        <ActionButton
          icon={OpenFolderIcon}
          alt="Open folder"
          onClick={openFolder}
        />
        {_type === "bark" && (
          <ActionButton
            icon={RecordVoiceOverIcon}
            alt="Use as voice"
            onClick={useAsVoice}
          />
        )}
      </div>
    );
  };

  const MetadataBlock = ({
    prompt,
    language,
    history_hash,
    ...rest
  }: Omit<GenerationRaw, "filename" | "date">) => {
    return _type === "tortoise" ? (
      <TortoiseMetadata
        language={language || "unknown"}
        history_hash={history_hash}
        {...rest}
      />
    ) : (
      <Metadata
        prompt={prompt}
        language={language || "unknown"}
        history_hash={history_hash}
        {...rest}
      />
    );
  };

  return (
    <div className="flex flex-col items-center justify-start w-full max-w-md py-4 px-6 bg-white rounded shadow-lg">
      <div className="flex flex-col gap-y-4 w-full h-full">
        <div className="flex w-full">
          <h1 className="text-2xl font-bold text-gray-900">
            <span
              className={
                promptText.length > maxLength
                  ? "text-xl font-bold text-gray-900"
                  : "text-2xl font-bold text-gray-900"
              }
            >
              {promptText.length > maxLength
                ? promptText.substring(0, maxLength) + "..."
                : promptText}
            </span>
          </h1>
        </div>
        <div className="flex w-full justify-between items-center">
          <AudioPlayer audio={absoluteFilename} />
          <p className="text-gray-500 ml-2">{prettifyDate(date, true)}</p>
        </div>
        <ActionRow
          isFavorite={isFavorite}
          removeFavorite={removeFavorite}
          addFavorite={addFavorite}
          saveToVoices={saveToVoices}
          openFolder={openFolder}
          useAsVoice={useAsVoice}
          filename={absoluteFilename}
          _type={_type}
        />
        <div className="flex flex-col text-gray-500">
          <p className="text-gray-500 text-sm">{folder_root}</p>
          <Row label="Model" value={_type} />
        </div>
        <MetadataBlock
          prompt={prompt}
          language={language || "unknown"}
          history_hash={history_hash}
          {...rest}
        />
      </div>
    </div>
  );
};

const TortoiseMetadata = ({
  language,
  history_hash,
  ...rest
}: {
  language: string;
  history_hash: string;
  [key: string]: any;
}) => {
  return (
    <div className="text-xs text-gray-500 flex flex-col w-full break-words">
      {/* render everything as rows */}
      <div className="flex flex-col">
        <Row label="Language" value={language} />
        <Row label="History Hash" value={history_hash} />
        {Object.entries(rest).map(([key, value]) => (
          <Row key={key} label={key} value={value} />
        ))}
      </div>
    </div>
  );
};

type FullNPZ = NPZ & NPZOptional;

function isFullNPZ(generation: NPZ): generation is FullNPZ {
  return (generation as FullNPZ).prompt !== undefined;
}

export const CardVoiceNpz = ({ generation }: { generation: NPZ }) => {
  const { filename, date, url } = generation;
  const image = "/" + url.replace(".npz", ".png");

  const Extra = () => {
    if (!isFullNPZ(generation)) return <></>;
    return (
      <div className="flex flex-col gap-y-2 w-full h-full justify-between">
        <div className="flex w-full justify-between">
          <p className="text-gray-500">{prettifyDate(date!)}</p>
        </div>
        <details>
          <summary>Metadata</summary>
          <Metadata {...generation} />
        </details>
      </div>
    );
  };

  const useAsVoice = () => {
    const history_npz = filename;
    sendToBarkAsVoice(history_npz);
  };

  const preview = filename.replace(".npz", ".wav");

  const name = filename
    .replace(".npz", "")
    .replace("voices/", "")
    // reformat date YYYY-MM-DD to YYYY/MM/DD
    // .replace(/(\d{4})-(\d{2})-(\d{2})/, "$1/$2/$3")
    .replace(/(\d{4})-(\d{2})-(\d{2})/, "")
    // reformat time HH-MM-SS to HH:MM:SS
    // .replace(/(\d{2})-(\d{2})-(\d{2})/, "$1:$2:$3")
    .replace(/(\d{2})-(\d{2})-(\d{2})/, "")
    .replaceAll("_", " ")
    .replaceAll("-", " ");
  return (
    <div className="flex flex-col items-center justify-start w-full max-w-md py-4 px-6 bg-white rounded shadow-lg">
      {image && (
        <img
          className="w-24 h-24 rounded select-none"
          src={image}
          width={96}
          height={96}
          alt="voice"
          onError={(e) => {
            const target = e.target as HTMLImageElement;
            target.onerror = null;
            target.src =
              "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=";
            target.width = 0;
            target.height = 0;
          }}
        />
      )}
      <p className=" text-lg w-full ">{name}</p>
      <div className="flex w-full justify-between">
        {/* <AudioPlayer audio={preview} /> */}
        {/* <Download download={getWebuiURLWithHost(filename)} /> */}
        {/* delete */}

        <Button
          variant="outline"
          size="default"
          onClick={async () => {
            const urlWithHost = getWebuiURLWithHost(filename);
            const x = await encodecDecode({
              npz_file: toLocalCacheFile(urlWithHost),
            });
            const audio = new Audio(x.url);
            audio.play();
          }}
        >
          Listen
          <PlayCircleIcon className="ml-2 w-5 h-5" />
        </Button>
        <Button variant="outline" size="default" onClick={useAsVoice}>
          Run
          <SendHorizontalIcon className="ml-2 w-5 h-5" />
        </Button>
      </div>
      <Extra />
    </div>
  );
};

const HASH_OF_NONE = "6adf97f83acf6453d4a6a4b1070f3754"; // == md5("None")
export const SectionVoice = ({
  generation: {
    name,
    prompt,
    language,
    history_hash,
    filename,
    date,
    hash,
    ...rest
  },
  children,
}: {
  generation: GenerationRaw;
  children: React.ReactNode;
}) => {
  // Detect if prompt is Japanese
  if (!prompt) return <></>;
  const isJapanese = prompt.match(/[\u3040-\u309F\u30A0-\u30FF]/);
  const maxLength = isJapanese ? 30 : 50;
  const promptText =
    prompt.length > maxLength ? prompt.substring(0, maxLength) + "..." : prompt;
  const title = name || promptText;
  // const maxLength = 100000;
  return (
    <div className="flex flex-col items-center justify-start w-full py-4 px-6 bg-white rounded shadow-lg">
      <div className="flex flex-col gap-y-4 w-full h-full justify-between">
        <div className="flex w-full">
          <h1 className="text-2xl font-bold text-gray-900">
            <span
              className={
                name === undefined && prompt.length > maxLength
                  ? "text-xl font-bold text-gray-900"
                  : "text-2xl font-bold text-gray-900"
              }
            >
              {title}
            </span>
          </h1>
        </div>
        <div className="flex w-full justify-between">
          {hash !== HASH_OF_NONE ? (
            <Download download={filename} />
          ) : (
            <div></div>
          )}
          <p className="text-gray-500">{prettifyDate(date)}</p>
        </div>
        <Metadata
          prompt={prompt}
          language={language || "unknown"}
          history_hash={history_hash}
          hash={hash}
          {...rest}
        />
      </div>
      <h1 className="text-2xl font-bold text-gray-900">Used in generations:</h1>
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
        {children}
      </div>
    </div>
  );
};

export const CardEmpty = ({ title, link }: { title: string; link: string }) => {
  return (
    <div className="flex flex-col items-center justify-start w-full max-w-md py-4 px-6 bg-white rounded shadow-lg">
      <div className="flex flex-col gap-y-4 w-full h-full justify-between">
        <div className="flex items-center w-full gap-x-2">
          <h1 className="text-2xl font-bold text-gray-900">{title}</h1>
        </div>
        <div className="flex w-full justify-center">
          <button
            className="w-24 h-24 rounded bg-gray-200 hover:bg-gray-300"
            onClick={() => window.open(link, "_blank")}
          >
            <MUIIcon
              icon={AddIcon}
              alt="Add a new voice"
              className="w-24 h-24 rounded"
            />
          </button>
        </div>
        <div className="flex w-full justify-between">
          <div className="w-8 h-8" />
        </div>
      </div>
    </div>
  );
};

const Author = ({ author }: Pick<Voice, "author">) => {
  return (
    <a
      className="hover:opacity-50 w-9 h-9 flex items-center justify-center select-none"
      href={`https://github.com/${author}`}
      target="_blank"
      title={`Author: ${author}`}
    >
      <img
        src={AuthorIcon.src}
        width={AuthorIcon.width}
        height={AuthorIcon.height}
        alt="author"
      />
    </a>
  );
};

const Heart = () => <>&#x2764;</>;
const Trash = () => <>X</>;

const SaveToFavorites = ({ download }: Pick<Voice, "download">) => {
  const [favorites, setFavorites] = useFavorites();
  const isFavorite = favorites.includes(download);
  return (
    <button
      className={`w-9 h-9 text-xl text-gray-900 select-none ${
        isFavorite ? "text-red-500 hover:text-red-900" : "hover:text-red-500"
      }`}
      onClick={() =>
        saveOrDeleteFromFavorites(favorites, isFavorite, download, setFavorites)
      }
      title={isFavorite ? "Remove from favorites" : "Add to favorites"}
    >
      <Heart />
    </button>
  );
};

const AudioPlayer = ({ audio }: Pick<Voice, "audio">) => {
  const [isPlaying, setIsPlaying] = useState(false);
  const audioRef = useRef<HTMLAudioElement>(null);

  const handlePlayPause = () => {
    const audio = audioRef.current;

    if (!audio) {
      return;
    }

    if (isPlaying) {
      audio.pause();
    } else {
      audio.play();
    }

    setIsPlaying(!isPlaying);
  };

  // set is playing to false when audio ends
  useEffect(() => {
    const audio = audioRef.current;

    if (!audio) {
      return;
    }

    const handleEnded = () => {
      setIsPlaying(false);
    };

    audio.addEventListener("ended", handleEnded);

    return () => {
      audio.removeEventListener("ended", handleEnded);
    };
    // }, [audioRef.current]);
  }, []); // because it's a ref, it doesn't need to be in the dependency array

  return (
    <div className="flex items-center">
      <button
        className="-mx-2 w-8 h-8 hover:opacity-50"
        onClick={handlePlayPause}
      >
        <img
          src={isPlaying ? PauseIcon.src : PlayIcon.src}
          width={isPlaying ? PauseIcon.width : PlayIcon.width}
          height={isPlaying ? PauseIcon.height : PlayIcon.height}
          alt={isPlaying ? "Pause" : "Play"}
          className="select-none w-full h-full"
        />
      </button>
      <audio
        ref={audioRef}
        src={audio}
        //  prevent preload
        preload="none"
      />
    </div>
  );
};

const Download = ({ download }: Pick<Voice, "download">) => (
  <a
    className="w-8 h-8 hover:opacity-50 flex items-center justify-center select-none"
    href={download}
    title="Download .npz file"
    download
  >
    <img
      src={DownloadIcon.src}
      width={DownloadIcon.width}
      height={DownloadIcon.height}
      alt="download"
      className="select-none w-full h-full"
    />
  </a>
);

const genderToIcon = {
  male: ManIcon,
  female: WomanIcon,
  other: OtherIcon,
};

const pink = "-70deg";
const blue = "180deg";
const purple = "210deg";
const genderToHue = {
  male: blue,
  female: pink,
  other: purple,
};

const Gender = ({ gender }: Pick<Voice, "gender">) => {
  const iconData = genderToIcon[gender] || genderToIcon.other;
  const color = genderToHue[gender];
  const filter = `invert(1) brightness(0.5) sepia(1) hue-rotate(${color}) saturate(3)`;
  return (
    <button className="select-none" title={`Gender: ${gender}`}>
      <img
        style={{
          filter,
        }}
        src={iconData.src}
        width={iconData.width}
        height={iconData.height}
        alt="gender"
      />
    </button>
  );
};

const Tags = ({ tags }: Pick<Voice, "tags">) => (
  <ul className="flex flex-wrap justify-start items-start content-start gap-2">
    {tags.map((tag) => (
      <li
        key={tag}
        className="px-2 py-1 text-sm font-medium text-gray-800 bg-gray-200 rounded"
      >
        {tag}
      </li>
    ))}
  </ul>
);

const prettifyDate = (date: string, showTime = false) => {
  const dateObj = parseMetadataDate(date);
  return (
    <time dateTime={date}>
      {dateObj.toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",

        hour: showTime ? "numeric" : undefined,
        minute: showTime ? "numeric" : undefined,
      })}
    </time>
  );
};
