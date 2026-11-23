/// <reference types="vite/types/importMeta.d.ts" />

/**
 * The environment variables the app reads (src/data/env.ts).
 * Vite inlines them at build time.
 */
interface ImportMetaEnv {
  readonly VITE_DB_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

/**
 * An imported image file is `{ src, width, height }`: the icon components read
 * the size from it. The svgAsAsset plugin in vite.config.ts builds that
 * object for `.svg` files.
 */
declare module "*.svg" {
  const image: { src: string; width: number; height: number };
  export default image;
}
