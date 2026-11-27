import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import {
  RemixSlot,
  decodeToMono,
  encodeWav,
  remix,
} from "../../functions/remix";

const COLUMNS = 3;
const SLOTS_PER_COLUMN = 3;

type Named = { name: string; samples: Float32Array };

/**
 * Simple Remixer: mixes the slots of each column, joins the columns, in the
 * browser. Nothing is uploaded.
 */
const SimpleRemixerPage = () => {
  const contextRef = React.useRef<AudioContext | null>(null);
  const [input, setInput] = React.useState<Named | null>(null);
  const [slots, setSlots] = React.useState<(Named | null)[]>(
    Array(COLUMNS * SLOTS_PER_COLUMN).fill(null)
  );
  const [outputUrl, setOutputUrl] = React.useState("");
  const [outputSamples, setOutputSamples] = React.useState<Float32Array | null>(
    null
  );
  const [error, setError] = React.useState("");

  const context = () => {
    if (!contextRef.current) contextRef.current = new AudioContext();
    return contextRef.current;
  };

  React.useEffect(
    () => () => {
      if (outputUrl) URL.revokeObjectURL(outputUrl);
    },
    [outputUrl]
  );

  const load = async (file: File | undefined): Promise<Named | null> => {
    if (!file) return null;
    setError("");
    try {
      return { name: file.name, samples: await decodeToMono(context(), file) };
    } catch {
      setError(`Could not decode ${file.name}`);
      return null;
    }
  };

  const setSlot = (i: number, value: Named | null) =>
    setSlots((prev) => prev.map((s, j) => (j === i ? value : s)));

  const publish = (samples: Float32Array) => {
    setOutputSamples(samples);
    setOutputUrl(URL.createObjectURL(encodeWav(samples, context().sampleRate)));
  };

  const concatenate = () => {
    const columns: RemixSlot[][] = Array.from({ length: COLUMNS }, (_, c) =>
      slots.slice(c * SLOTS_PER_COLUMN, (c + 1) * SLOTS_PER_COLUMN)
    );
    const result = remix(columns);
    if (!result) {
      setError("Nothing to concatenate: all slots are empty.");
      return;
    }
    setError("");
    publish(result);
  };

  const fileInput = (onFile: (f: File | undefined) => void) => (
    <input
      type="file"
      accept="audio/*"
      onChange={(e) => {
        onFile(e.target.files?.[0]);
        e.target.value = "";
      }}
    />
  );

  return (
    <Template title="Simple Remixer">
      <div className="flex flex-col gap-4 p-4">
        <p className="text-sm text-gray-600">
          The slots of one column are mixed together, then the three columns are
          joined one after the other.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        <div className="border rounded p-2 flex gap-2 items-center flex-wrap">
          <span className="font-medium">Input audio</span>
          {fileInput(async (f) => setInput(await load(f)))}
          <span className="text-sm">{input?.name ?? "empty"}</span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {Array.from({ length: COLUMNS }, (_, c) => (
            <div key={c} className="flex flex-col gap-2">
              {Array.from({ length: SLOTS_PER_COLUMN }, (_, r) => {
                const i = c * SLOTS_PER_COLUMN + r;
                return (
                  <div key={i} className="border rounded p-2 flex flex-col gap-2">
                    <span className="font-medium">Slot {c + 1}.{r + 1}</span>
                    {fileInput(async (f) => setSlot(i, await load(f)))}
                    <span className="text-sm">{slots[i]?.name ?? "empty"}</span>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!slots[i]}
                        onClick={() => setSlot(i, null)}
                      >
                        Clear
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!input}
                        onClick={() => setSlot(i, input)}
                      >
                        Copy from input
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
        <div className="flex gap-2">
          <Button onClick={concatenate}>Concatenate</Button>
        </div>
        {outputUrl && (
          <div className="flex flex-col gap-2">
            <span className="font-medium">Output audio</span>
            <audio controls src={outputUrl} className="w-full" />
            <div className="flex gap-2">
              <a className="underline" href={outputUrl} download="remix.wav">
                Download
              </a>
              <Button
                size="sm"
                variant="outline"
                onClick={() =>
                  outputSamples &&
                  setInput({ name: "remix.wav", samples: outputSamples })
                }
              >
                Send to input
              </Button>
            </div>
          </div>
        )}
      </div>
    </Template>
  );
};

export default SimpleRemixerPage;
