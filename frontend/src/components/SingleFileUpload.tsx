import React from "react";
import { getFilenameFromLocalFileURL, uploadFile } from "./FileInput";
import { FileUpIcon, PaperclipIcon } from "lucide-react";
import { Label } from "./ui/label";
import {
  FileUploader,
  FileUploaderContent,
  FileUploaderItem,
  FileInput,
} from "./ui/extension/file-upload";
import { cn } from "../lib/utils";

export const SingleFileUpload = ({
  label = "",
  callback,
  file,
  dropZoneConfig = {},
  className,
  accept,
  ...props
}: {
  label?: string;
  callback: (file?: string) => void;
  file?: string;
  accept?: {};
  dropZoneConfig?: any;
} & React.HTMLAttributes<HTMLDivElement>) => {
  const handleFile = async (files: File[] | null): Promise<any> =>
    callback(files?.[0] && (await uploadFile(files?.[0])));
  const dropZoneConfig_ = {
    maxFiles: 1,
    accept: accept || { "application/json": [".json"] },
    maxSize: 1024 * 1024 * 50,
    ...dropZoneConfig,
  };
  return (
    <FileUploader
      value={file ? [file as any] : []}
      onValueChange={handleFile}
      dropzoneOptions={dropZoneConfig_}
      //   className="relative bg-background cell h-36 items-center p-4"
      className={cn(
        "relative bg-background cell h-36 items-center p-4",
        className
      )}
      {...props}
    >
      {file ? (
        <Label className="text-center text-md">{label}:</Label>
      ) : (
        <FileInput className="outline-dashed outline-1 outline-white">
          <div className="flex flex-col items-center justify-center p-3 w-full text-gray-500 dark:text-gray-400">
            <FileUpIcon className="w-8 h-8 mb-2" />
            <Label className="text-center text-md font-bold">{label}:</Label>
            <p className="mb-1 text-sm">
              <span className="font-semibold">Click to upload</span>
              &nbsp;or drag and drop&nbsp;
            </p>
            <p className="text-xs">
              Types:{" "}
              {Object.values(dropZoneConfig_.accept)
                .join(",")
                .toUpperCase()
                .replace(/,/g, ", ")
                .replace(/\./g, "")}
            </p>
          </div>
        </FileInput>
      )}
      <FileUploaderContent>
        {file && (
          <FileUploaderItem index={0}>
            <PaperclipIcon className="h-4 w-4 stroke-current" />
            <span>
              {getFilenameFromLocalFileURL(file)?.slice(0, 32)}
              {file.length > 32 && "..."}
            </span>
          </FileUploaderItem>
        )}
      </FileUploaderContent>
    </FileUploader>
  );
};
