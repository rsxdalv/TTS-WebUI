/// <reference types="vite/client" />

/**
 * The environment variables the Vite build reads (src/data/env.vite.ts).
 * Vite inlines them at build time.
 */
interface ImportMetaEnv {
  readonly TTS_STATIC_EXPORT?: string;
  readonly NEXT_PUBLIC_DB_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
