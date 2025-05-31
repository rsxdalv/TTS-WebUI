import { useEffect, useState } from "react";
import WaveSurfer, { WaveSurferOptions } from "wavesurfer.js";

export const useWavesurfer = (containerRef, options: WaveSurferOptions) => {
  const [wavesurfer, setWavesurfer] = useState<WaveSurfer | null>(null);

  // Initialize wavesurfer when the container mounts
  // or any of the props change
  useEffect(() => {
    if (!containerRef.current) return;

    const ws = WaveSurfer.create({
      ...options,
      height: 50,
      url: undefined,
      container: containerRef.current,
    });
    ws.load(options.url!).catch((e) => {
      console.log("=== Wavesurfer loading failed", e);
    });
    setWavesurfer(ws);
    return () => {
      ws.destroy();
    };
  }, [options, containerRef]);

  return wavesurfer;
};
