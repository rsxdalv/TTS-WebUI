import useLocalStorage from "../hooks/useLocalStorage";

const initialHistory = []; // prevent infinite loop

export const useHistory = <T extends any>(prefix: string) =>
  useLocalStorage<T[]>(prefix + "GenerationHistory", initialHistory);
