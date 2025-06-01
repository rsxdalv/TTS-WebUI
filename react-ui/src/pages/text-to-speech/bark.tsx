import { ProjectCard } from "@/components/ProjectCard";
import { Template } from "@/components/Template";

export const AudioConversionModelList = () => (
  <div className="flex flex-col gap-2 text-center max-w-2xl">
    {/* <h3 className="text-xl font-bold">Audio Conversion Models:</h3>
    <ProjectCard
      title="RVC"
      description="An easy-to-use voice conversion framework based on VITS."
      href="/audio-conversion/rvc"
    />
    <ProjectCard
      title="Demucs"
      description="Demucs is a post-processing model for Music Source Separation."
      href="/audio-conversion/demucs"
    />
    <ProjectCard
      title="Vocos Wav"
      description="Vocos Wav is a post-processing model that can refine the output of a text-to-speech model."
      href="/audio-conversion/vocos_wav"
    />
    <ProjectCard
      title="Vocos NPZ"
      description="Vocos NPZ is a post-processing model that can refine the output of a Bark."
      href="/text-to-speech/bark/vocos_npz"
    /> */}
    <h3 className="text-xl font-bold">Bark:</h3>
    <ProjectCard
      title="Generation"
      description="Generate audio from text."
      href="/text-to-speech/bark/generation"
    />
    <ProjectCard
      title="Voice Generation"
      description="Generate a voice from audio."
      href="/text-to-speech/bark/bark_voice_generation"
    />
    <ProjectCard
      title="Voices"
      description="Manage your Bark voices."
      href="/text-to-speech/bark/voices"
    />
    <ProjectCard
      title="Settings"
      description="Configure Bark settings."
      href="/text-to-speech/bark/bark_settings"
    />
    <ProjectCard
      title="Vocos NPZ"
      description="Vocos NPZ is a post-processing model that can refine the output of a Bark."
      href="/text-to-speech/bark/vocos_npz"
    />
    <h3 className="text-xl font-bold">More Voices:</h3>
    <ProjectCard
      title="PromptEcho"
      description="A collection of voices for Bark."
      href="https://promptecho.com/?utm_source=react_ui"
      target="_blank"
    />
    <ProjectCard
      title="Bark Speaker Directory"
      description="A collection of voices for Bark."
      href="https://rsxdalv.github.io/bark-speaker-directory/"
      target="_blank"
    />
  </div>
);

export default function BarkMainPage() {
  return (
    <Template title="Bark">
      <AudioConversionModelList />
    </Template>
  );
}
