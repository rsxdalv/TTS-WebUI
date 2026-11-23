import React, { useState } from "react";
import { Template } from "../../components/Template";
import { AudioInput } from "../../components/AudioComponents";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { BarkMetadataDisplay } from "../../components/BarkMetadataDisplay";

const FFMPEGMetadataPage = () => {
  const [audioUrl, setAudioUrl] = useState<string | undefined>();
  const [metadata, setMetadata] = useState<any>(null);

  const loadMetadata = async (x: string) => {
    // x is /file-input-cache/<name>; the Go core reads that uploaded file.
    const name = decodeURIComponent(x.split("/").pop() ?? "");
    const response = await fetch("/api/go-data/ffmpeg-metadata", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    const data = await response.json();
    setMetadata(response.ok ? data : { error: data.error ?? response.statusText });
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
              callback={(file) => {
                setAudioUrl(file);
                setMetadata(null);
                if (!file) return;
                loadMetadata(file);
              }}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Metadata</CardTitle>
          </CardHeader>
          <CardContent>
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
