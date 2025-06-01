import { ProjectCard } from "../components/ProjectCard";
import { Template } from "../components/Template";

export const TextToSpeechModelList = () => (
  <div className="flex flex-col gap-2 text-center max-w-2xl">
    <h3 className="text-xl font-bold">Text-to-Speech Models:</h3>
    <ProjectCard
      title="Kokoro"
      description="Kokoro is a fast and lightweight TTS model with 82 million parameters. Small but comparable in quality to larger models."
      href="/text-to-speech/kokoro"
      projectLink="https://github.com/hexgrad/kokoro"
    />
    <ProjectCard
      title="Bark"
      description="Bark is a text-to-speech model that can generate speech from text."
      href="/text-to-speech/bark/generation"
      projectLink="https://github.com/suno-ai/bark"
    />
    <ProjectCard
      title="Tortoise"
      description="Tortoise is a text-to-speech model that can generate speech from text."
      href="/text-to-speech/tortoise"
      projectLink="https://github.com/neonbjb/tortoise-tts"
    />
    <ProjectCard
      title="Maha TTS"
      description="Maha TTS is a text-to-speech model that can generate speech from text, supports many Indian languages."
      href="/text-to-speech/maha-tts"
      projectLink="https://github.com/dubverse-ai/MahaTTS"
    />
    <ProjectCard
      title="MMS"
      description="Fairseq based text-to-speech model that supports 1000+ languages"
      href="/text-to-speech/mms"
      projectLink="https://github.com/facebookresearch/fairseq/blob/main/examples/mms/README.md"
    />
    <ProjectCard
      title="VALL-E X"
      description="Multilingual TTS: Speak in three languages - English, Chinese, and Japanese - with natural and expressive speech synthesis."
      href="/text-to-speech/vallex"
      projectLink="https://github.com/Plachtaa/VALL-E-X"
    />
  </div>
);

export default function TextToSpeech() {
  return (
    <Template title="Text to Speech">
      <TextToSpeechModelList />
    </Template>
  );
}
