export const applySeed = <
  T extends { use_random_seed?: boolean; seed?: number }
>(
  params: T
) =>
  params.use_random_seed
    ? { ...params, seed: getRandomSeed() }
    : params;

const getRandomSeed = () => Math.floor(Math.random() * 2 ** 32 - 1);
