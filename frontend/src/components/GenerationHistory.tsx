import React from "react";
import { AudioOutput } from "./AudioComponents";
import useLocalStorage from "../hooks/useLocalStorage";
import { Button } from "./ui/button";

export const GenerationHistorySimple = ({
  setHistoryData,
  name,
  ...props
}: {
  setHistoryData: React.Dispatch<React.SetStateAction<any[]>>;
  name: string;
  // pass through props
  historyData: any[];
  funcs?: Record<string, (audio: string | undefined | any) => void>;
  nameKey?: string;
  filter?: string[];
  getLabel?: (item: any) => string;
}) => {
  const [showLast, setShowLast] = useLocalStorage<number>(
    name + "ShowLast",
    10
  );

  return (
    <GenerationHistory
      clearHistory={() => setHistoryData((x) => x.slice(0, 1))}
      showLast={showLast}
      setShowLast={setShowLast}
      {...props}
    />
  );
};

export const GenerationHistory = ({
  clearHistory,
  showLast,
  setShowLast,
  historyData,
  funcs,
  nameKey,
  getLabel,
  filter,
}: {
  clearHistory: () => void;
  showLast: number;
  setShowLast: React.Dispatch<React.SetStateAction<number>>;
  historyData: any[];
  funcs?: Record<string, (audio: string | undefined | any) => void>;
  nameKey?: string; // deprecated
  getLabel?: (item: any) => string;
  filter?: string[];
}) => (
  <div className="flex flex-col gap-y-2 cell">
    <div className="flex gap-x-2">
      <label className="text-md mr-auto">History</label>
      {/* <button
        className="border border-gray-300 py-2 px-12 rounded"
        onClick={clearHistory}
      >
        Clear History
      </button> */}
      <Button variant="outline" size="sm" onClick={clearHistory}>
        Clear History
      </Button>
      <div className="flex gap-x-2 items-center">
        <label className="text-sm">Show Last X entries:</label>
        <input
          type="number"
          value={showLast}
          onChange={(event) => setShowLast(Number(event.target.value))}
          className="cell"
          min="0"
          max="100"
          step="1"
        />
      </div>
    </div>
    <div className="flex flex-col gap-8">
      {/* {JSON.stringify(historyData)} */}
      {historyData &&
        historyData
          .slice(1, showLast + 1)
          .map((item, index) => (
            <AudioOutput
              key={index}
              audioOutput={item.audio}
              metadata={item}
              label={
                getLabel
                  ? getLabel(item)
                  : nameKey
                    ? item[nameKey]
                    : `History ${index}`
              }
              funcs={funcs}
              filter={filter}
            />
          ))}
    </div>
  </div>
);
