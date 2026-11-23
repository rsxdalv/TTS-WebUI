/**
 * The environment variables of the app.
 *
 * Next inlines `process.env.*` at build time. Vite inlines
 * `import.meta.env.*` instead, so the Vite build uses
 * src/data/env.vite.ts (see the alias in vite.config.ts) and the Next build
 * uses this file. Both give the shared sources the same values.
 */

/** True when the build is a static export (npm run build:static). */
export const isStaticExport = (): boolean =>
  process.env.TTS_STATIC_EXPORT === "1";

/**
 * True when the page data comes from the API instead of the server render.
 *
 * The server build has the data in its props (getStaticProps), so it only
 * asks the API when the Go proxy says it answers (the tts-go-claims
 * cookie). A static export has no props, so it always asks.
 */
export const dataComesFromApi = (): boolean => isStaticExport();

/** The URL of the database API, when it is not the default. */
export const databaseApiUrl = (): string =>
  process.env.NEXT_PUBLIC_DB_API_URL || "";
