export const remove_use_random_seed = <
  T extends { use_random_seed?: boolean }
>({
  use_random_seed,
  ...params
}: T) => params;
