import { ProjectCard } from "../components/ProjectCard";
import { Template } from "../components/Template";

export const AudioMusicGenerationModelList = () => (
  <div className="flex flex-col gap-2 text-center max-w-2xl">
    <h3 className="text-xl font-bold">Audio/Music Generation Models:</h3>
    <ProjectCard
      title="Musicgen"
      description="MusicGen is a state-of-the-art controllable text-to-music model."
      href="/audio-music-generation/musicgen"
      projectLink="https://github.com/facebookresearch/audiocraft"
    />
    <ProjectCard
      title="MAGNeT"
      description="A state-of-the-art non-autoregressive model for text-to-music and text-to-sound."
      href="/audio-music-generation/magnet"
      projectLink="https://github.com/facebookresearch/audiocraft"
    />
    <ProjectCard
      title="Stable Audio"
      description="A state-of-the-art non-autoregressive model for text-to-music and text-to-sound."
      href="/audio-music-generation/stable-audio"
      projectLink="https://github.com/facebookresearch/audiocraft"
    />
    <ProjectCard
      title="ACE-Step"
      description="ACE-Step: A Step Towards Music Generation Foundation Model."
      href="/audio-music-generation/ace-step"
      projectLink="https://github.com/ACE-Step/ACE-Step"
    />
  </div>
);

export default function AudioMusicGeneration() {
  return (
    <Template title="Audio Music Generation">
      <AudioMusicGenerationModelList />
    </Template>
  );
}
