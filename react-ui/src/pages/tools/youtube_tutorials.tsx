import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";

const VIDEOS = [
  { title: "Chat with Chatterbox in SillyTavern!", id: "_0rftbXPJLI" },
  { title: "Chatterbox TTS", id: "yJhzwmwFpcs" },
  { title: "Extensions", id: "nfZEoXOGX5Y" },
  { title: "ACE-Step Demo", id: "HFtrCnczZQI" },
  { title: "Bark Voice Generation", id: "4LqoEy3BM_0" },
];

/** The tutorial videos of the youtube_tutorials extension. */
const YouTubeTutorialsPage = () => {
  const [index, setIndex] = React.useState(0);
  const video = VIDEOS[index];
  return (
    <Template title="YouTube Tutorials">
      <div className="flex flex-col gap-3 p-4">
        <div className="flex flex-wrap gap-2">
          {VIDEOS.map((v, i) => (
            <Button
              key={v.id}
              variant={i === index ? "default" : "outline"}
              onClick={() => setIndex(i)}
            >
              {v.title}
            </Button>
          ))}
        </div>
        <iframe
          key={video.id}
          width="100%"
          height="500"
          src={`https://www.youtube-nocookie.com/embed/${video.id}`}
          title={video.title}
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
          allowFullScreen
        />
      </div>
    </Template>
  );
};

export default YouTubeTutorialsPage;
