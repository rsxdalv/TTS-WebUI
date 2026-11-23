/**
 * The environment variables of the app, for the Vite build.
 *
 * Vite inlines `import.meta.env.*` at build time (see the envPrefix in
 * vite.config.ts). The Next build uses src/data/env.ts instead.
 */

/** True when the build is a static export (npm run build:static). */
export const isStaticExport = (): boolean =>
  import.meta.env.TTS_STATIC_EXPORT === "1";

/**
 * True when the page data comes from the API instead of the server render.
 *
 * The Vite build has no server render: the browser gets the pages from the
 * bundle and the data from the API, so the data always comes from the API.
 */
export const dataComesFromApi = (): boolean => true;

/** The URL of the database API, when it is not the default. */
export const databaseApiUrl = (): string =>
  import.meta.env.NEXT_PUBLIC_DB_API_URL || "";
