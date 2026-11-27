"use client";
import { Template } from "@/components/Template";
import VoiceConfigGenerator from "./_api_config";

export default function VoiceConfigGeneratorPage() {
  return (
    <Template title="API Config">
      <VoiceConfigGenerator />
    </Template>
  );
}
