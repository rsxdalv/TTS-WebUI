import fs from "fs";
import path from "path";
// music-metadata v8: "node" condition (lib/index.js) has parseFile, but TS resolves
// the "default" condition (lib/core.js) which is browser-only. Works fine at runtime.
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { parseFile } from "music-metadata";
import { parseMetadataDate } from "../components/parseMetadataDate";
import * as AdmZip from "adm-zip";
import { parseNpy } from "./npz-parsing";
import { npyToUtf8 } from "./npyToUtf8";
import { getWebuiURL } from "./getWebuiURL";
import { baseUrlPath } from "./baseUrlPath";

// In Next.js 16 (Turbopack), __dirname is virtualized and may not contain ".next".
// Use process.cwd() (the Next.js project root) to reliably compute paths instead.
const __next__base__dirname = process.cwd();
const basePath = path.join(__next__base__dirname, "public");
export const webuiBasePath = path.join(__next__base__dirname, "..");

const getWebuiPath = (dir: string) => path.join(webuiBasePath, dir);

const oggPath = getWebuiPath("favorites");
const getOgg = () => fs.readdirSync(oggPath);

const npzPath = path.join(basePath, "voice-drafts");
const npzPathWebui = getWebuiPath("voices");
const getNpzs = (dir: string = npzPath) => fs.readdirSync(dir);

export const getOggData = async (collection = "favorites") => {
  const ogg = getOgg();
  // TODO - filter tortoise?
  const oggData = ogg.map(async (dirname) => {
    const coreFilename = path.join(dirname, dirname + ".ogg");
    const filename = path.join(oggPath, coreFilename);
    const metadata = await parseFile(filename);
    try {
      const result = JSON.parse(
        // metadata?.common?.comment?.[0] || "{}"
        metadata?.native?.vorbis?.filter((x) => x.id === "DESCRIPTION")[0]
          .value || "{}"
      );
      return generateResult(result, collection, coreFilename, dirname);
    } catch (error) {
      console.error(error);
      console.log("Error parsing metadata for file: " + filename);
      try {
        console.log(
          "metadata?.native?.vorbis?.filter((x) => x.id === 'DESCRIPTION')[0].value: " +
            metadata?.native?.vorbis?.filter((x) => x.id === "DESCRIPTION")[0]
              .value
        );
      } catch (error) {
        console.error(error);
      }
      return null;
    }
  });

  const oggDataParsed = (await Promise.all(oggData))
    .filter((x) => x !== null)
    .filter((x) => x.date);
  // Sort by date
  oggDataParsed.sort((a, b) => {
    return (
      parseMetadataDate(b.date).getTime() - parseMetadataDate(a.date).getTime()
    );
  });
  return oggDataParsed;
};

const parseToNpzData = (buf: Buffer | ArrayBuffer, filename?: string) => {
  try {
    return new AdmZip.default(buf instanceof Buffer ? buf : Buffer.from(buf))
      .getEntries()
      .filter((entry) => entry.name === "metadata.npy")
      .map((entry) => parseNpy(entry.getData().buffer))
      .map((entry) => npyToUtf8(entry))
      .map((entry) => JSON.parse(entry))[0];
  } catch (error) {
    console.log("Error parsing metadata for file: " + filename);
    console.error(error);
    return null;
  }
};

export const getNpzData = async () =>
  getNpzs().map((npz) => ({
    ...parseToNpzData(fs.readFileSync(path.join(npzPath, npz))),
    filename: path
      .join(baseUrlPath, "voice-drafts", npz)
      .split(path.sep)
      .join("/"),
  }));

export const getNpzDataSimpleVoices = async () => {
  let files: string[] = [];
  try {
    files = getNpzs(npzPathWebui);
  } catch {
    // Directory may not exist in build environments
    return [];
  }
  return files
    .filter((file) => file.endsWith(".npz"))
    .map((npz) => ({
      ...parseToNpzData(fs.readFileSync(path.join(npzPathWebui, npz)), npz),
      filename: path.join(baseUrlPath, "voices", npz).split(path.sep).join("/"),
      url: getWebuiURL("voices", npz),
    }));
};

export const getDataFromJSON = async (collection = "outputs") => {
  const basePath = getWebuiPath(collection);
  const dirs = await fs.promises.readdir(basePath).catch((error) => {
    console.error(error);
    return [];
  });

  const oggData = dirs.map(async (dirname) => {
    const coreFilename = path.join(dirname, dirname + ".ogg");
    const jsonFilename = path.join(basePath, dirname, dirname + ".json");
    try {
      const json = await fs.promises.readFile(jsonFilename, "utf-8");
      const result = JSON.parse(json);
      return generateResult(result, collection, coreFilename, dirname);
    } catch (error) {
      console.error(error);
      console.log("Error parsing metadata for file: " + jsonFilename);
      return null;
    }
  });

  const oggDataParsed = (await Promise.all(oggData))
    .filter((x) => x !== null)
    .filter((x) => x.date);
  // Sort by date
  oggDataParsed.sort((a, b) => {
    return (
      parseMetadataDate(b.date).getTime() - parseMetadataDate(a.date).getTime()
    );
  });
  return oggDataParsed;
};

const generateResult = (
  result: any,
  collection: string,
  coreFilename: string,
  dirname: string
) => ({
  ...result,
  semantic_prompt: null,
  coarse_prompt: null,

  filename: getWebuiURL(collection, coreFilename),

  folder_root: path.join(collection, dirname),
  api_filename: path.join(collection, coreFilename),
});
