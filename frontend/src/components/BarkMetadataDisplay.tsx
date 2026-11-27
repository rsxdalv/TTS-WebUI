"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";

interface BarkMetadataDisplayProps {
  data: {
    _version: string;
    _hash_version: string;
    _type: string;
    is_big_semantic_model: boolean;
    is_big_coarse_model: boolean;
    is_big_fine_model: boolean;
    prompt: string;
    language: string | null;
    speaker_id: string | null;
    hash: string;
    history_prompt: string;
    history_prompt_npz: string | null;
    history_hash: string;
    text_temp: number;
    waveform_temp: number;
    date: string;
    seed: string;
    semantic_prompt: string;
    coarse_prompt: string;
  };
}

const YesNoBadge = ({ value }: { value: boolean }) => (
  <Badge variant={value ? "default" : "secondary"} className="ml-2">
    {value ? "Yes" : "No"}
  </Badge>
);

const Entry = ({ label, value }: { label: string; value: React.ReactNode }) => (
  <p className="space-x-2">
    <span className="font-medium">{label}:</span>
    <span>{value}</span>
  </p>
);

const SectionHeader = ({ children }: { children: React.ReactNode }) => (
  <h3 className="text-lg font-semibold mb-2">{children}</h3>
);

const ModelSubInformation = ({
  data,
}: {
  data: BarkMetadataDisplayProps["data"];
}) => (
  <div className="space-y-2">
    <Entry
      label="Big Semantic Model"
      value={<YesNoBadge value={data.is_big_semantic_model} />}
    />
    <Entry
      label="Big Coarse Model"
      value={<YesNoBadge value={data.is_big_coarse_model} />}
    />
    <Entry
      label="Big Fine Model"
      value={<YesNoBadge value={data.is_big_fine_model} />}
    />
  </div>
);

export function BarkMetadataDisplay({ data }: BarkMetadataDisplayProps) {
  return (
    <Card className="w-full max-w-3xl mx-auto">
      <CardHeader>
        <CardTitle className="text-2xl font-bold">
          Bark Metadata Display
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <SectionHeader>General Information</SectionHeader>
            <div className="space-y-2">
              <Entry label="Version" value={data._version} />
              <Entry label="Hash Version" value={data._hash_version} />
              <Entry label="Type" value={data._type} />
              <Entry label="Date" value={data.date} />
              <Entry label="Seed" value={data.seed} />
            </div>
          </div>
          <div>
            <SectionHeader>Model Information</SectionHeader>
            <ModelSubInformation data={data} />
          </div>
        </div>
        <div className="mt-4">
          <SectionHeader>Prompt Information</SectionHeader>
          <div className="space-y-2">
            <Entry label="Prompt" value={data.prompt} />
            <Entry label="Language" value={data.language || "Not specified"} />
            <Entry
              label="Speaker ID"
              value={data.speaker_id || "Not specified"}
            />
          </div>
        </div>
        <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <SectionHeader>Hash Information</SectionHeader>
            <div className="space-y-2">
              <Entry label="Hash" value={data.hash} />
              <Entry label="History Hash" value={data.history_hash} />
            </div>
          </div>
          <div>
            <SectionHeader>Temperature</SectionHeader>
            <div className="space-y-2">
              <Entry label="Text Temperature" value={data.text_temp} />
              <Entry label="Waveform Temperature" value={data.waveform_temp} />
            </div>
          </div>
        </div>
        <div className="mt-4">
          <SectionHeader>Prompts</SectionHeader>
          <div className="space-y-2">
            <div>
              <p className="font-medium">Semantic Prompt:</p>
              <ScrollArea className="h-20 w-full rounded-md border p-2">
                {data.semantic_prompt}
              </ScrollArea>
            </div>
            <div>
              <p className="font-medium">Coarse Prompt:</p>
              <ScrollArea className="h-20 w-full rounded-md border p-2">
                {data.coarse_prompt}
              </ScrollArea>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
