import { Inter } from "next/font/google";
import React from "react";

import { Template } from "../components/Template";
import Head from "next/head";
import { ProjectCard } from "../components/ProjectCard";
import { TextToSpeechModelList } from "./text-to-speech";
import { AudioMusicGenerationModelList } from "./audio-music-generation";
import { AudioConversionModelList } from "./audio-conversion";

export const inter = Inter({ subsets: ["latin"] });

export default function Home() {
  return (
    <Template>
      <Head>
        <title>TTS WebUI</title>
      </Head>
      <div className="flex flex-col gap-2 text-center max-w-2xl">
        <h2 className="text-2xl font-bold">Welcome to the TTS Webui!</h2>
        <p className="text-lg">
          This is a web interface for the TTS project. It allows you to generate
          audio using the TTS models.
        </p>
        <p className="text-lg">
          To get started, select a tab above to choose a model and generate some
          audio.
        </p>
        <TextToSpeechModelList />
        <AudioMusicGenerationModelList />
        <AudioConversionModelList />
      </div>
    </Template>
  );
}

export const getStaticProps = async () => {
  return {
    props: {},
  };
};
