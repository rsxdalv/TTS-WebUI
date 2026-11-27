export const useSeedHelper =
  <T extends { seed: number }>(
    setParams: React.Dispatch<React.SetStateAction<T>>
  ) =>
  (
    _url: string,
    data?: {
      metadata: {
        seed: string | number;
      };
    }
  ) => {
    const seed = data?.metadata?.seed;
    if (!seed) return;
    setParams((x) => ({ ...x, seed: Number(seed), use_random_seed: false }));
  };
