import { Template } from "../components/Template";
import { ProjectCard } from "../components/ProjectCard";

export const AudioConversionModelList = () => (
  <div className="flex flex-col gap-2 text-center max-w-2xl">
    <h3 className="text-xl font-bold">Audio Conversion Models:</h3>
    <ProjectCard
      title="RVC"
      description="An easy-to-use voice conversion framework based on VITS."
      href="/audio-conversion/rvc"
      projectLink="https://github.com/RVC-Project/Retrieval-based-Voice-Conversion-WebUI"
    />
    <ProjectCard
      title="Demucs"
      description="Demucs is a post-processing model for Music Source Separation."
      href="/audio-conversion/demucs"
      projectLink="https://github.com/facebookresearch/demucs"
    />
    <ProjectCard
      title="Vocos Wav"
      description="Vocos Wav is a post-processing model that can refine the output of a text-to-speech model."
      href="/audio-conversion/vocos_wav"
      projectLink="https://github.com/gemelo-ai/vocos"
    />
    <ProjectCard
      title="Vocos NPZ"
      description="Vocos NPZ is a post-processing model that can refine the output of a Bark."
      href="/text-to-speech/bark/vocos_npz"
      projectLink="https://github.com/gemelo-ai/vocos"
    />
  </div>
);

export default function AudioConversion() {
  return (
    <Template title="Audio Conversion">
      <AudioConversionModelList />
    </Template>
  );
}
