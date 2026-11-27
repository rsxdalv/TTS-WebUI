export const extractTexts = (
  text: string,
  params: { iterations?: number; splitByLines: any }
) => (params.splitByLines ? text.split("\n") : [text]);

export const incrementNonRandomSeed = (seed: number, iteration: number) =>
  seed === -1 ? -1 : seed + iteration;

export const getMax = (texts: string[], iterations: number) =>
  texts.length * iterations;

export const initialHyperParams = {
  iterations: 1,
  splitByLines: false,
};
