/**
 * The browser version of src/data/getVoicesData.tsx.
 *
 * The original reads the voice and generation data from the file system, so
 * it only works on the server. In the browser the data comes from the API
 * instead (see src/data/useGoData.ts), so these functions are the ones the
 * browser build uses: they return empty data.
 *
 * The Vite build swaps the module for this one (see the alias in
 * vite.config.ts). The Next build keeps the original.
 */

export const getOggData = async (_collection = "favorites") => [];

export const getNpzData = async () => [];

export const getNpzDataSimpleVoices = async () => [];

export const getDataFromJSON = async (_collection = "outputs") => [];

export const webuiBasePath = "";
