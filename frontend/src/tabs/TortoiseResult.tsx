import { GradioFile } from "../types/GradioFile";


export type TortoiseResult = {
  audio: GradioFile;
  image: string;
  seed: string;
  bundle_name: string;
  metadata: {
    // Potentially wrong
    _version: string;
    _hash_version: string;
    _type: string;
    text: string;
    voice: string;
    seed: string;
    name: string;
    split_prompt: boolean;
    cvvp_amount: number;
    preset: string;
    candidates: number;
    date: string;
  };
};
