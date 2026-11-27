import React from "react";
import { goGetJson } from "../functions/goData";
import { HostStatus, launcherUrl } from "../functions/hostStatus";

/**
 * "The Extensions page of the launcher", as a link to the launcher this page
 * was started by (its port comes from the host status, not from a guess). Plain
 * text until the port is known.
 */
export const LauncherExtensionsLink = ({
  text = "Extensions page of the launcher",
}: {
  text?: string;
}) => {
  const [url, setUrl] = React.useState("");
  React.useEffect(() => {
    goGetJson<HostStatus>("/api/go-data/host-status")
      .then((h) => setUrl(launcherUrl(h)))
      .catch(() => undefined);
  }, []);
  return url ? (
    <a className="underline" href={url + "#extensions"}>
      {text}
    </a>
  ) : (
    <>{text}</>
  );
};
