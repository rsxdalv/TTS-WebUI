/**
 * The environment variables of the app. Vite inlines `import.meta.env.*` at
 * build time.
 */

/** The URL of the database API, when it is not the default. */
export const databaseApiUrl = (): string =>
  import.meta.env.VITE_DB_API_URL || "";
