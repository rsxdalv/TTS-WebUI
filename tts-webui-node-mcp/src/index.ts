#!/usr/bin/env node
import fs from "fs/promises";
import path from "path";

import { Client } from "@gradio/client";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const GRADIO_BACKEND =
  process.env.GRADIO_BACKEND ||
  process.env.GRADIO_BACKEND_AUTOMATIC ||
  "http://127.0.0.1:7770/";

const GRADIO_AUTH = process.env.GRADIO_AUTH?.includes(":")
  ? (process.env.GRADIO_AUTH.split(":") as [string, string])
  : undefined;

const server = new McpServer({
  name: "tts-webui-node-mcp",
  version: "0.1.0",
});

const asTextContent = (payload: unknown) => ({
  content: [
    {
      type: "text",
      text:
        typeof payload === "string"
          ? payload
          : JSON.stringify(payload, null, 2),
    },
  ],
});

async function getClient() {
  return Client.connect(GRADIO_BACKEND, { auth: GRADIO_AUTH });
}

async function resolveFile(source?: string | null) {
  if (!source) return null;
  if (source.startsWith("data:")) {
    const match = source.match(/^data:([^;]+);base64,(.+)$/);
    if (!match) {
      throw new Error("Invalid data URI for file input");
    }
    return Buffer.from(match[2], "base64");
  }
  if (source.startsWith("http")) {
    const resp = await fetch(source);
    if (!resp.ok) {
      throw new Error(`Failed to fetch file: ${resp.status} ${resp.statusText}`);
    }
    return Buffer.from(await resp.arrayBuffer());
  }
  const filePath = path.isAbsolute(source) ? source : path.resolve(source);
  return fs.readFile(filePath);
}

const ttsSchema = z.object({
  text: z.string(),
  voice: z.string().default("af_heart"),
  model_name: z.string().default("hexgrad/Kokoro-82M"),
  speed: z.number().default(1),
  seed: z.number().default(0),
  use_gpu: z.boolean().default(true),
});

server.registerTool(
  "tts_generate",
  {
    description:
      "Generate speech via TTS WebUI kokoro endpoint (text, voice, speed, seed, model_name).",
    inputSchema: ttsSchema,
  },
  async (input) => {
    const params = ttsSchema.parse(input);
    const client = await getClient();
    const result = await client.predict("/kokoro", params as any);
    const [audio, tokens, metadata, folder_root] = (result as any).data ?? [];

    return asTextContent({
      audio: audio?.url ?? audio,
      tokens,
      metadata,
      folder_root,
      backend: GRADIO_BACKEND,
    });
  }
);

const stableAudioSchema = z.object({
  text: z.string(),
  negative_prompt: z.string().default(""),
  seconds_start: z.number().default(0),
  seconds_total: z.number().default(60),
  cfg_scale: z.number().default(7),
  steps: z.number().default(100),
  preview_every: z.number().default(0),
  sampler_type: z.string().default("dpmpp-3m-sde"),
  sigma_min: z.number().default(0.03),
  sigma_max: z.number().default(500),
  cfg_rescale: z.number().default(0),
  use_init: z.boolean().default(false),
  init_audio: z.string().optional(),
  init_noise_level: z.number().default(0.1),
  seed: z.number().default(0),
});

server.registerTool(
  "music_generate",
  {
    description:
      "Generate music/audio via stable_audio_generate (supports prompt, negative_prompt, seed, init_audio).",
    inputSchema: stableAudioSchema,
  },
  async (input) => {
    const params = stableAudioSchema.parse(input);
    const client = await getClient();
    const initAudio = await resolveFile(params.init_audio);

    const { text, init_audio, ...rest } = params;
    const payload = {
      prompt: text,
      init_audio: initAudio,
      ...rest,
    } as Record<string, unknown>;

    const result = await client.predict("/stable_audio_generate", payload);
    const [audio, gallery] = (result as any).data ?? [];

    return asTextContent({
      audio: audio?.url ?? audio,
      gallery,
      backend: GRADIO_BACKEND,
    });
  }
);

const enhanceSchema = z.object({
  audio: z.string(),
  bandwidth: z.string().default("1.5"),
});

server.registerTool(
  "enhance_audio",
  {
    description: "Enhance audio with Vocos (bandwidth adjustment).",
    inputSchema: enhanceSchema,
  },
  async (input) => {
    const params = enhanceSchema.parse(input);
    const audioFile = await resolveFile(params.audio);
    if (!audioFile) {
      throw new Error("Audio input is required");
    }
    const client = await getClient();
    const result = await client.predict("/vocos_wav", {
      audio: audioFile,
      bandwidth: params.bandwidth,
    });
    const enhanced = (result as any).data?.[0];

    return asTextContent({
      audio: enhanced?.url ?? enhanced,
      backend: GRADIO_BACKEND,
    });
  }
);

const sttSchema = z.object({
  audio: z.string(),
  model_name: z.string().default("openai/whisper-large-v3"),
});

server.registerTool(
  "stt_transcribe",
  {
    description: "Transcribe audio via the Whisper extension (whisper_transcribe endpoint).",
    inputSchema: sttSchema,
  },
  async (input) => {
    const params = sttSchema.parse(input);
    const audioFile = await resolveFile(params.audio);
    if (!audioFile) {
      throw new Error("Audio input is required");
    }
    const client = await getClient();
    const result = await client.predict("/whisper_transcribe", [
      audioFile,
      params.model_name,
    ]);
    const [transcript] = (result as any).data ?? [];

    return asTextContent({
      transcript,
      backend: GRADIO_BACKEND,
    });
  }
);

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("tts-webui-node-mcp server running on stdio");
}

main().catch((error) => {
  console.error("Server error:", error);
  process.exit(1);
});
