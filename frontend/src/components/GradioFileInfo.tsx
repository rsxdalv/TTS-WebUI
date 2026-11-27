import React from "react";
import { GradioFile } from "../types/GradioFile";

export const GradioFileInfo = ({ audioOutput }: { audioOutput: GradioFile; }) => (
  <div className="cell">
    <p className="text-sm">Info</p>
    <div className="flex flex-col text-xs">
      <div className="flex flex-row">
        <div className="w-1/3">Name</div>
        <div className="w-1/3">{audioOutput.name}</div>
      </div>
      <div className="flex flex-row">
        <div className="w-1/3">Original name</div>
        <div className="w-1/3">{audioOutput.orig_name}</div>
      </div>
      <div className="flex flex-row">
        <div className="w-1/3">Size</div>
        <div className="w-1/3">{audioOutput.size}</div>
      </div>
      <div className="flex flex-row">
        <div className="w-1/3">Is file</div>
        <div className="w-1/3">{audioOutput.is_file ? "true" : "false"}</div>
      </div>
    </div>
  </div>
);
