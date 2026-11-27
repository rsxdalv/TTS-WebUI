import useLocalStorage, {
  readLocalStorage,
  updateLocalStorageWithFunction,
} from "../hooks/useLocalStorage";
import router from "@/shims/router";
import { GradioFile } from "../types/GradioFile";
import { splitWithDemucs } from "../functions/splitWithDemucs";
import { useHistory } from "../hooks/useHistory";
import { LocalCacheFile, toLocalCacheFile } from "../types/LocalCacheFile";

export type DemucsParams = {
  audio?: LocalCacheFile;
};

export const initialState: DemucsParams = {
  audio: toLocalCacheFile(
    "https://www.mfiles.co.uk/mp3-downloads/gs-cd-track2.mp3"
  ),
};

export type DemucsGradioFile = GradioFile & {
  type_name?: string;
};

type DemucsResult = {
  audio: DemucsGradioFile;
  typeName: string;
};

const typeNames = ["drums", "bass", "other", "vocals"];

const addTypeNamesToAudioOutputs = (
  audioOutputs: DemucsGradioFile[],
  typeNames: string[]
) =>
  audioOutputs.map((audioOutput, index) => ({
    ...audioOutput,
    type_name: typeNames[index],
  }));

const demucsId = "demucs-tab.v2";

export const sendToDemucs = (file?: string) => {
  if (!file) return;
  updateLocalStorageWithFunction(
    demucsId,
    (demucsParams: DemucsParams = initialState) => ({
      ...demucsParams,
      audio: toLocalCacheFile(file),
    })
  );
  router.push("/demucs");
};

export const useDemucsParams = () =>
  useLocalStorage<DemucsParams>(demucsId, initialState);

export const useDemucsResults = () =>
  useLocalStorage<DemucsParams | null>(demucsId + ".output", null);

export const getDemucsParams = (): DemucsParams =>
  readLocalStorage(demucsId) ?? initialState;

export const useDemucsPage = () => {
  const [demucsParams, setDemucsParams] = useDemucsParams();

  const [historyData, setHistoryData] = useHistory<DemucsResult>("demucs");

  const consumer = async (params: DemucsParams) => {
    const result = await splitWithDemucs(params);
    const sampleWithTypeNames = addTypeNamesToAudioOutputs(result, typeNames);
    const data = typeNames.map((typeName) => ({
      audio: sampleWithTypeNames?.find((item) => item.type_name === typeName)!,
      typeName,
    }));
    setHistoryData((historyData) => [...data, ...historyData]);
  };

  const funcs = {
    useAsInput: (audio?: string) => {
      if (!audio) return;
      setDemucsParams({ ...demucsParams, audio: toLocalCacheFile(audio) });
    },
  };

  return {
    demucsParams,
    setDemucsParams,
    historyData,
    setHistoryData,
    consumer,
    funcs,
  };
};
