import React from "react";
import { Template } from "../../components/Template";
import Head from "next/head";
import { ExtensionHuggingFaceCacheManager } from "../../components/ExtensionHuggingFaceCacheManager";

const GPUInfoPage = () => {
  return (
    <Template>
      <Head>
        <title>GPU Info - TTS WebUI</title>
      </Head>
      <div className="gap-y-4 p-4 flex w-full flex-col items-center">
        <ExtensionHuggingFaceCacheManager />
      </div>
    </Template>
  );
};

export default GPUInfoPage;
