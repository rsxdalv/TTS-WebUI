
export type Voice = {
  name: string;
  image: string;
  audio: string;
  download: string;
  tags: string[];
  language: "american" | "german" | "spanish" | "french" | "hindi" | "chinese" | "portuguese" |"russian" | "turkish" | "polish" | "korean" | "japanese" | "italian";
  author: string;
  gender: "male" | "female" | "other";
};
