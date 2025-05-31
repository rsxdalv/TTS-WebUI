import React from "react";
import Link from "next/link";
import { useRouter } from "next/router";
import { ExternalLinkIcon } from "lucide-react";
import { RouteCommands } from "./RouteCommands";
import { cn } from "../lib/utils";

export type Route = {
  href: string;
  text: string | React.ReactNode;
  target?: string;
  subroutes?: Route[];
};

export const routes: Route[] = [
  {
    href: "/",
    text: "Home",
  },
  {
    href: "/pipeline",
    text: "Pipeline",
  },
  {
    href: "/text-to-speech",
    text: "Text-to-Speech",
    subroutes: [
      {
        href: "/text-to-speech/bark",
        text: "Bark",
        subroutes: [
          {
            href: "/text-to-speech/bark/generation",
            text: "Generation",
          },
          {
            href: "/text-to-speech/bark/bark_voice_generation",
            text: "Voice Generation",
          },
          {
            href: "/text-to-speech/bark/voices",
            text: "Voices",
          },
          {
            href: "/text-to-speech/bark/bark_settings",
            text: "Settings",
          },
          {
            href: "/text-to-speech/bark/vocos_npz",
            text: "Vocos NPZ",
          },
          {
            href: "https://promptecho.com/?utm_source=react_ui",
            text: (
              <span className="inline-flex items-center">
                More Voices&nbsp;
                <ExternalLinkIcon className="inline-block w-4 h-4" />
              </span>
            ),
            target: "_blank",
          },
        ],
      },
      {
        href: "/text-to-speech/tortoise",
        text: "Tortoise",
      },
      {
        href: "/text-to-speech/maha-tts",
        text: "Maha TTS",
      },
      {
        href: "/text-to-speech/mms",
        text: "MMS",
      },
      {
        href: "/text-to-speech/vallex",
        text: "Vall-E-X",
      },
      {
        href: "/text-to-speech/kokoro",
        text: "Kokoro",
      },
    ],
  },
  {
    href: "/audio-music-generation",
    text: "Audio/Music Generation",
    subroutes: [
      {
        href: "/audio-music-generation/musicgen",
        text: "MusicGen",
      },
      {
        href: "/audio-music-generation/magnet",
        text: "MAGNeT",
      },
      {
        href: "/audio-music-generation/stable-audio",
        text: "Stable Audio",
      },
      {
        href: "/audio-music-generation/ace-step",
        text: "ACE-Step",
      },
    ],
  },
  {
    href: "/audio-conversion",
    text: "Audio Conversion",
    subroutes: [
      {
        href: "/audio-conversion/rvc",
        text: "RVC",
      },
      {
        href: "/audio-conversion/demucs",
        text: "Demucs",
      },
      {
        href: "/audio-conversion/vocos_wav",
        text: "Vocos Wav",
      },
    ],
  },
  {
    href: "/outputs",
    text: "Outputs",
    subroutes: [
      {
        href: "/outputs/outputs",
        text: "History (Slow!)",
      },
      {
        href: "/outputs/favorites",
        text: "Favorites (Slow!)",
      },
      // {
      //   href: "/outputs/generations",
      //   text: "Generations View (Beta)",
      // },
      // {
      //   href: "/outputs/voice-drafts",
      //   text: "Voice Tree",
      // },
      {
        href: "/outputs/ffmpeg-metadata-page",
        text: "FFMPEG Metadata",
      },
      // {
      //   href: "/outputs/collections",
      //   text: "History Collections",
      // },
    ],
  },
  {
    href: "/tools",
    text: "Tools",
    subroutes: [
      {
        href: "/tools/gpu_info",
        text: "GPU Info",
      },
      {
        href: "/extensions/huggingface_cache_manager",
        text: "Huggingface Cache Manager",
      },
    ],
  },
];

const RouteList = ({ children }: { children: React.ReactNode }) => (
  <div className="flex gap-4 items-center justify-center w-full p-2 bg-white rounded shadow-sm">
    {children}
  </div>
);

const RouteLinks = ({ routes }: { routes: Route[] }) => {
  const router = useRouter();
  const route = router.asPath.replace("/", "");
  const currentTopRoute =
    routes.find((r) => r.href.slice(1) === route) ||
    routes.find(
      (r) => r.href.slice(1) === route.slice(0, route.lastIndexOf("/"))
    ) ||
    routes.find((r) => r.href.slice(1) === route.slice(0, route.indexOf("/")));
  const currentInnerRoute =
    currentTopRoute?.subroutes?.find((r) => r.href.slice(1) === route) ||
    currentTopRoute?.subroutes?.find(
      (r) => r.href.slice(1) === route.slice(0, route.lastIndexOf("/"))
    );

  const LinkHelper = ({
    href,
    text,
    target,
    highlighted = false,
  }: {
    href: string;
    text: React.ReactNode;
    target?: string;
    highlighted?: boolean;
  }) => (
    <Link
      href={href}
      className={cn(highlighted ? "font-bold" : "hover:text-gray-400")}
      target={target}
    >
      {text}
    </Link>
  );

  const subroutes = currentTopRoute?.subroutes;
  const innerSubroutes = currentInnerRoute?.subroutes;

  // route === href.slice(1)

  return (
    <>
      <RouteList>
        {routes.map(({ href, text, target }, i) => (
          <LinkHelper
            key={i}
            href={href}
            text={text}
            target={target}
            highlighted={currentTopRoute?.href === href}
          />
        ))}
      </RouteList>
      {subroutes && (
        <RouteList>
          {subroutes.map(({ href, text, target }, i) => (
            <LinkHelper
              key={i}
              href={href}
              text={text}
              target={target}
              highlighted={currentInnerRoute?.href === href}
            />
          ))}
        </RouteList>
      )}
      {innerSubroutes && (
        <RouteList>
          {innerSubroutes.map(({ href, text }, i) => (
            <LinkHelper
              key={i}
              href={href}
              text={text}
              highlighted={href === router.asPath}
            />
          ))}
        </RouteList>
      )}
    </>
  );
};

export const Header = ({}) => {
  return (
    <>
      <div className="flex items-center w-full pb-1">
        <h1 className="text-3xl font-bold text-start w-full text-gray-900">
          TTS Generation Webui
        </h1>
        <div className="mr-2 relative w-96 flex-shrink-0 h-12 z-10">
          <div className="w-full">
            <RouteCommands />
          </div>
        </div>
        <a
          href="https://github.com/rsxdalv/tts-webui"
          target="_blank"
          className="text-gray-500 hover:underline"
        >
          GitHub
        </a>
        &nbsp; &nbsp;
        <a
          href="https://forms.gle/2L62owhBsGFzdFBC8"
          target="_blank"
          className="text-gray-500 hover:underline whitespace-pre"
        >
          Feedback / Bug reports
        </a>
      </div>
      <RouteLinks routes={routes} />
    </>
  );
};
