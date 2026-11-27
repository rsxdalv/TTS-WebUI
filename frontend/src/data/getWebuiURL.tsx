import path from "path";
import { baseUrlPath } from "./baseUrlPath";

export const addHost = (url: string) => `${window.location.origin}/${url}`;

export const getWebuiURL = (...args: string[]) =>
  path
    .join(baseUrlPath, "api", "webui-generations", ...args)
    .split(path.sep)
    .join("/");

export const getWebuiURLWithHost = (...args: string[]) =>
  addHost(getWebuiURL(...args));
