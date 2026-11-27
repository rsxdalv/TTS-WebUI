import React from "react";
import { Button } from "./ui/button";

export const DownloadButton = ({
  url,
  name = "audio",
}: {
  url?: string;
  name?: string;
}) => {
  const [downloadURL, setDownloadURL] = React.useState<string | undefined>(
    undefined
  );

  React.useEffect(() => {
    if (!url) return;
    const download = (url) => {
      if (!url) {
        throw new Error("Resource URL not provided! You need to provide one");
      }
      fetch(url)
        .then((response) => response.blob())
        .then((blob) => {
          const blobURL = URL.createObjectURL(blob);
          setDownloadURL(blobURL);
        })
        .catch((e) => console.log("=== Error downloading", e));
    };
    download(url);
  }, [url]);

  return (
    <Button
      variant="ghost"
      size="icon"
      asChild
      className="flex-shrink-0 h-8 w-8"
    >
      <a className="cursor-pointer" href={downloadURL} download={`${name}.wav`}>
        {/* <DownloadIcon className="w-5 h-5" /> */}
        {/* <button className="p-1 hover:bg-gray-100 rounded"> */}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="currentColor"
          className="w-5 h-5"
        >
          <path
            fillRule="evenodd"
            d="M12 2.25a.75.75 0 0 1 .75.75v11.69l3.22-3.22a.75.75 0 1 1 1.06 1.06l-4.5 4.5a.75.75 0 0 1-1.06 0l-4.5-4.5a.75.75 0 1 1 1.06-1.06l3.22 3.22V3a.75.75 0 0 1 .75-.75Zm-9 13.5a.75.75 0 0 1 .75.75v2.25a1.5 1.5 0 0 0 1.5 1.5h13.5a1.5 1.5 0 0 0 1.5-1.5V16.5a.75.75 0 0 1 1.5 0v2.25a3 3 0 0 1-3 3H5.25a3 3 0 0 1-3-3V16.5a.75.75 0 0 1 .75-.75Z"
            clipRule="evenodd"
          />
        </svg>
        {/* </button> */}
      </a>
    </Button>
  );
};
