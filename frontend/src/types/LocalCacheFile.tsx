const TAG = "local_cache_file";

export type LocalCacheFile = {
  __type: typeof TAG;
  path: string;
};

export const isLocalCacheFile = (x: any): x is LocalCacheFile =>
  typeof x === "object" && x.__type === TAG;

export const toLocalCacheFile = (path?: string): LocalCacheFile | undefined =>
  path
    ? {
        __type: TAG,
        path,
      }
    : undefined;
