import React, { useEffect, useRef, useState, useCallback } from "react";
import { useWavesurfer } from "@/components/useWavesurfer";
import Timeline from "wavesurfer.js/dist/plugins/timeline";
import { WaveSurferOptions } from "wavesurfer.js";

interface EnhancedWaveSurferProps extends Omit<WaveSurferOptions, "container"> {
  volume: number;
  show_controls?: boolean;
  autoplay?: boolean;
  loop?: boolean;
  editable?: boolean;
  show_download_button?: boolean;
  onPlay?: () => void;
  onPause?: () => void;
  onStop?: () => void;
  onEdit?: () => void;
  onError?: (error: any) => void;
}

const WaveSurferPlayerRaw = (props) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const wavesurfer = useWavesurfer(containerRef, props);

  // On play button click
  // const onPlayClick = useCallback(() => {
  //   if (!wavesurfer) return;
  //   wavesurfer.isPlaying() ? wavesurfer.pause() : wavesurfer.play();
  // }, [wavesurfer]);
  // Initialize wavesurfer when the container mounts
  // or any of the props change
  useEffect(() => {
    if (!wavesurfer) return;

    setCurrentTime(0);
    setIsPlaying(false);

    wavesurfer.setVolume(props.volume);

    const subscriptions = [
      wavesurfer.on("play", () => {
        setIsPlaying(true);
        props.onPlay?.();
      }),
      wavesurfer.on("pause", () => {
        setIsPlaying(false);
        props.onPause?.();
      }),
      wavesurfer.on("timeupdate", (currentTime) => setCurrentTime(currentTime)),
      wavesurfer.on("finish", () => {
        setIsPlaying(false);
        if (props.loop) {
          wavesurfer.play();
        }
      }),
      wavesurfer.on("destroy", () => setIsPlaying(false)),
      // wavesurfer.on("ready", () => {
      //   setDuration(wavesurfer.getDuration());
      // }),
      wavesurfer.on("error", (error) => {
        console.error("WaveSurfer error:", error);
        props.onError?.(error);
      }),
    ];

    return () => {
      subscriptions.forEach((unsub) => unsub());
    };
  }, [wavesurfer]); // eslint-disable-line react-hooks/exhaustive-deps

  // const { volume } = props;
  // useEffect(() => {
  //   if (!wavesurfer) return;
  //   wavesurfer.setVolume(volume);
  // }, [volume]);

  const onPlayClick = useCallback(() => {
    if (!wavesurfer) return;
    wavesurfer.isPlaying() ? wavesurfer.pause() : wavesurfer.play();
  }, [wavesurfer]);

  const onStopClick = useCallback(() => {
    if (!wavesurfer) return;
    wavesurfer.stop();
    props.onStop?.();
  }, [wavesurfer, props.onStop]);

  const goToStart = useCallback(() => {
    if (!wavesurfer) return;
    wavesurfer.seekTo(0);
  }, [wavesurfer]);

  const goToEnd = useCallback(() => {
    if (!wavesurfer) return;
    wavesurfer.seekTo(1);
  }, [wavesurfer]);

  // Don't render controls if show_controls is false
  const showControls = props.show_controls !== false;

  return (
    <>
      <div ref={containerRef} style={{ height: props.height + 20 }} />

      {showControls && (
        <div className="w-full justify-center flex items-center gap-2 mt-2 text-gray-500">
          {/* Skip to start */}
          <button onClick={goToStart} className="p-1 hover:bg-gray-100 rounded">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="currentColor"
              className="w-6 h-6"
            >
              <path d="M9.195 18.44c1.25.714 2.805-.189 2.805-1.629v-2.34l6.945 3.968c1.25.715 2.805-.188 2.805-1.628V8.69c0-1.44-1.555-2.343-2.805-1.628L12 11.029v-2.34c0-1.44-1.555-2.343-2.805-1.628l-7.108 4.061c-1.26.72-1.26 2.536 0 3.256l7.108 4.061Z" />
            </svg>
          </button>

          {/* Play/Pause */}
          <button
            onClick={onPlayClick}
            className="p-1 hover:bg-gray-100 rounded"
          >
            {isPlaying ? (
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="w-8 h-8"
              >
                <path
                  fillRule="evenodd"
                  d="M6.75 5.25a.75.75 0 0 1 .75-.75H9a.75.75 0 0 1 .75.75v13.5a.75.75 0 0 1-.75.75H7.5a.75.75 0 0 1-.75-.75V5.25Zm7.5 0A.75.75 0 0 1 15 4.5h1.5a.75.75 0 0 1 .75.75v13.5a.75.75 0 0 1-.75.75H15a.75.75 0 0 1-.75-.75V5.25Z"
                  clipRule="evenodd"
                />
              </svg>
            ) : (
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="w-8 h-8"
              >
                <path
                  fillRule="evenodd"
                  d="M4.5 5.653c0-1.427 1.529-2.33 2.779-1.643l11.54 6.347c1.295.712 1.295 2.573 0 3.286L7.28 19.99c-1.25.687-2.779-.217-2.779-1.643V5.653Z"
                  clipRule="evenodd"
                />
              </svg>
            )}
          </button>

          {/* Skip to end */}
          <button onClick={goToEnd} className="p-1 hover:bg-gray-100 rounded">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="currentColor"
              className="w-6 h-6"
            >
              <path d="M5.055 7.06C3.805 6.347 2.25 7.25 2.25 8.69v8.122c0 1.44 1.555 2.343 2.805 1.628L12 14.471v2.34c0 1.44 1.555 2.343 2.805 1.628l7.108-4.061c1.26-.72 1.26-2.536 0-3.256l-7.108-4.061C13.555 6.346 12 7.249 12 8.689v2.34L5.055 7.061Z" />
            </svg>
          </button>

          {/* Download button (if enabled) */}
          {props.show_download_button && (
            <button className="p-1 hover:bg-gray-100 rounded ml-2">
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
            </button>
          )}
        </div>
      )}
    </>
  );
};

// memoize the player component
export const MemoizedWaveSurferPlayer = React.memo(WaveSurferPlayerRaw);
export const AudioPlayer = (
  props: Omit<WaveSurferOptions, "container"> & {
    volume: number;
    // sendAudioTo: (audio: string | undefined) => void;
  }
) => {
  const [plugins, setPlugins] = useState<any[]>([]);
  useEffect(() => {
    const topTimeline = Timeline.create({
      height: 15,
      insertPosition: "beforebegin",
      timeInterval: 0.2,
      primaryLabelInterval: 0,
      secondaryLabelInterval: 1,
      formatTimeCallback: (seconds) => "",
      style: {
        fontSize: "10px",
        color: "#2D5B88",
      },
    });

    // Create a second timeline
    const bottomTimeline = Timeline.create({
      height: 15,
      timeInterval: 0.5,
      primaryLabelInterval: 60,
      secondaryLabelInterval: 1,
      formatTimeCallback: (seconds) => {
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = Math.floor(seconds % 60);
        return `${minutes}:${remainingSeconds.toString().padStart(2, "0")}`;
      },
      style: {
        fontSize: "10px",
        color: "#2D5B88",
      },
    });

    setPlugins([topTimeline, bottomTimeline]);
  }, []);

  return <MemoizedWaveSurferPlayer {...props} plugins={plugins} />;
};
