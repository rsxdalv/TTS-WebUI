import React, { useEffect, useRef, useState } from "react";
import { Template } from "../../components/Template";
import { AudioInput } from "../../components/AudioComponents";
import { getFilenameFromLocalFileURL } from "../../components/FileInput";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { BarkMetadataDisplay } from "../../components/BarkMetadataDisplay";
import { errorText, goGetJson } from "../../functions/goData";
import { launcherUrl, type HostStatus } from "../../functions/hostStatus";

const FFMPEGMetadataPage = () => {
  const [audioUrl, setAudioUrl] = useState<string | undefined>();
  const [metadata, setMetadata] = useState<any>(null);
  const [missing, setMissing] = useState(false);
  const [launcher, setLauncher] = useState("");

  const request = useRef<AbortController | null>(null);

  useEffect(() => () => request.current?.abort(), []);

  const loadMetadata = async (x: string, signal: AbortSignal) => {
    // x is /file-input-cache/<name>; the Go core reads that uploaded file.
    const name = getFilenameFromLocalFileURL(x);
    const response = await fetch("/api/go-data/ffmpeg-metadata", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
      signal,
    });
    const data = await response.json();
    if (signal.aborted) return;
    setMetadata(response.ok ? data : { error: data?.error ?? response.statusText });
    if (response.status === 503) {
      // ffprobe is missing: the uv platform fetches ffmpeg; the launcher installs it.
      setMissing(true);
      const status = await goGetJson<HostStatus>("/api/go-data/host-status", signal)
        .catch(() => undefined);
      if (!signal.aborted) setLauncher(launcherUrl(status));
    }
  };

  const selectAudio = (file?: string) => {
    request.current?.abort();
    request.current = null;
    setAudioUrl(file);
    setMetadata(null);
    setMissing(false);
    setLauncher("");
    if (!file) return;

    const controller = new AbortController();
    request.current = controller;
    const { signal } = controller;
    loadMetadata(file, signal).catch((error) => {
      if (!signal.aborted) setMetadata({ error: errorText(error) });
    });
  };

  return (
    <Template title="FFMPEG Metadata">
      <div className="p-4 flex flex-col gap-4 w-full">
        <Card>
          <CardHeader>
            <CardTitle>Upload Audio File</CardTitle>
          </CardHeader>
          <CardContent>
            <AudioInput
              className="w-full"
              url={audioUrl}
              callback={selectAudio}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Metadata</CardTitle>
          </CardHeader>
          <CardContent>
            {missing && (
              <p className="mb-2">
                ffprobe is not installed. Installing any engine from the launcher installs uv first, which fetches ffmpeg:{" "}
                {launcher ? (
                  <a className="underline" href={launcher + "#extensions"}>
                    open the launcher's Extensions page
                  </a>
                ) : (
                  "open the launcher's Extensions page and install an engine (uv is installed first)"
                )}
                .
              </p>
            )}
            {metadata ? (
              <MetadataViewer metadata={metadata} />
            ) : (
              <p>No metadata loaded</p>
            )}
          </CardContent>
        </Card>
      </div>
    </Template>
  );
};

export default FFMPEGMetadataPage;

// function MetadataViewer(metadata: any): React.ReactNode {
//     return <pre className="overflow-x-scroll w-full bg-slate-100 p-2">
//         {/* {JSON.stringify(metadata, null, 2)} */}
//         {Object.entries(metadata).map(([key, value]) => (
//             <div key={key}>
//                 <b>{key}</b>:{JSON.stringify(value)}
//             </div>
//         ))}
//     </pre>;
// }

// as component
const MetadataViewer = ({ metadata }: { metadata: any }) => {
  if (metadata === null) return <p>No metadata loaded</p>;
  if (typeof metadata._type !== "string")
    return (
      <pre className="overflow-x-scroll w-full bg-slate-100 p-2">
        {JSON.stringify(metadata, null, 2)}
      </pre>
    );
  if (metadata._type === "bark") return <BarkMetadataDisplay data={metadata} />;
  return (
    <div className="overflow-x-scroll w-full bg-slate-100 p-2">
      {Object.entries(metadata).map(([key, value]) => (
        <div key={key}>
          <b>{key}</b>:&nbsp;{JSON.stringify(value)}
        </div>
      ))}
    </div>
  );
};
