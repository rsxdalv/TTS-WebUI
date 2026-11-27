import React from "react";
import { Template } from "../../components/Template";
import Head from "@/shims/head";
import { ExtensionHuggingFaceCacheManager } from "../../components/ExtensionHuggingFaceCacheManager";

const HuggingFaceCachePage = () => {
  return (
    <Template>
      <Head>
        <title>HuggingFace Cache - TTS WebUI</title>
      </Head>
      <div className="gap-y-4 p-4 flex w-full flex-col items-center">
        <ExtensionHuggingFaceCacheManager />
      </div>
    </Template>
  );
};

export default HuggingFaceCachePage;
