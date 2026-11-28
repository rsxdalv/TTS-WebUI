import React from "react";
import Link from "@/shims/link";
import { useRouter } from "@/shims/router";
import { ExternalLinkIcon } from "lucide-react";
import { RouteCommands } from "./RouteCommands";
import { ThemeToggle } from "./ThemeToggle";
import { cn } from "../lib/utils";
import {
  HostStatus,
  isGradioPath,
  launcherUrl,
  useHostStatus,
} from "../functions/hostStatus";
import { GradioTab, useGradioTabs } from "../functions/gradioTabs";

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
    href: "/chains",
    text: "Chains",
  },
  {
    href: "/text-to-speech",
    text: "Text-to-Speech",
    subroutes: [
      {
        href: "/text-to-speech/kokoro",
        text: "Kokoro",
      },
      {
        href: "/text-to-speech/chatterbox",
        text: "Chatterbox",
      },
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
            href: "https://ttswebui.com/promptecho/?utm_source=react_ui",
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
        href: "/text-to-speech/minimax-tts",
        text: "MiniMax Cloud TTS",
      },
    ],
  },
  {
    href: "/audio-music-generation",
    text: "Audio/Music Generation",
    subroutes: [
      {
        href: "/audio-music-generation/ace-step",
        text: "ACE-Step",
      },
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
    href: "/openai",
    text: "OpenAI",
  },
  {
    href: "/onnx",
    text: "ONNX",
  },
  {
    href: "/audiocpp",
    text: "audio.cpp",
  },
  {
    href: "/gradio",
    text: "Gradio (legacy)",
    // The tabs of the Gradio UI are added by useMenuRoutes (withGradioTabs).
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
      {
        href: "/tools/api_config",
        text: "API Config",
      },
      { href: "/tools/pip_list", text: "Installed Packages" },
      { href: "/tools/engines", text: "Engines" },
      { href: "/tools/extensions", text: "Extension Lists" },
      { href: "/tools/custom-extensions", text: "Custom Extensions Installer" },
      { href: "/tools/log_viewer", text: "Log Viewer" },
      { href: "/tools/model_location", text: "Model Location" },
      { href: "/tools/server_settings", text: "Server Settings" },
      { href: "/tools/youtube_tutorials", text: "YouTube Tutorials" },
    ],
  },
];

const RouteList = ({ children }: { children: React.ReactNode }) => (
  <div className="flex flex-wrap gap-x-4 gap-y-1 items-center justify-center w-full p-2 bg-white dark:bg-gray-900 rounded shadow-sm">
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
    routes.find((r) => r.href.slice(1) === route.slice(0, route.indexOf("/"))) ||
    // A page listed in a section's submenu whose URL does not start with the
    // section's (/extensions/huggingface_cache_manager under Tools).
    routes.find((r) => r.subroutes?.some((s) => s.href === `/${route}`));
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

/**
 * What the menu shows while the Gradio host is not installed: the pages that
 * call it are left out (a link to a page that cannot work is a dead link) and
 * one link points at the launcher, where the host is installed.
 */
const withoutGradioPages = (routes: Route[]): Route[] =>
  routes
    .filter((r) => !isGradioPath(r.href))
    .map((r) =>
      r.subroutes ? { ...r, subroutes: withoutGradioPages(r.subroutes) } : r
    );

const HostNotice = ({
  status,
  onGradioPage,
}: {
  status: HostStatus;
  onGradioPage: boolean;
}) => {
  const url = launcherUrl(status);
  return (
    <div className="w-full p-2 mt-1 bg-yellow-50 border border-yellow-300 rounded text-sm">
      {onGradioPage
        ? "This page needs the Gradio host, which is not installed. "
        : "The Gradio pages (text-to-speech, music, conversion, pipeline) need the Gradio host, which is not installed. "}
      {status.installable ? (
        url ? (
          <>
            Install it from the{" "}
            <a className="underline" href={url}>
              Extensions page of the launcher
            </a>
            .
          </>
        ) : (
          "Install it from the Extensions page of the launcher."
        )
      ) : (
        "It is managed outside this app."
      )}
    </div>
  );
};

/**
 * The Gradio entry lists the tabs of the Gradio UI as links: /gradio/<group>
 * for a top-level tab and /gradio/<group>/<tab> for a tab inside it.
 */
const withGradioTabs = (routes: Route[], tabs: GradioTab[] | null): Route[] =>
  !tabs
    ? routes
    : routes.map((r) =>
        r.href !== "/gradio"
          ? r
          : {
              ...r,
              subroutes: tabs.map((g) => ({
                href: `/gradio/${g.slug}`,
                text: g.label,
                subroutes: g.children.map((t) => ({
                  href: `/gradio/${g.slug}/${t.slug}`,
                  text: t.label,
                })),
              })),
            }
      );

/**
 * The routes the menu and the command palette offer: all of them, except the
 * pages that call the Gradio host while it is known not to be installed.
 */
export const useMenuRoutes = () => {
  const host = useHostStatus();
  const noHost = host.kind === "known" && !host.status.installed;
  const tabs = useGradioTabs(!noHost);
  const base = noHost ? withoutGradioPages(routes) : routes;
  return { host, noHost, routes: withGradioTabs(base, tabs) };
};

export const Header = ({}) => {
  const { host, noHost, routes: menuRoutes } = useMenuRoutes();
  const router = useRouter();
  return (
    <>
      <div className="flex items-center w-full pb-1">
        <h1 className="text-3xl font-bold text-start w-full text-gray-900 dark:text-gray-100">
          TTS WebUI
        </h1>
        <div className="mr-2 relative w-96 flex-shrink-0 h-12 z-10">
          <div className="w-full">
            <RouteCommands />
          </div>
        </div>
        <a
          href="https://github.com/rsxdalv/tts-webui"
          target="_blank"
          className="text-gray-600 hover:underline"
        >
          GitHub
        </a>
        &nbsp; &nbsp;
        <a
          href="https://forms.gle/2L62owhBsGFzdFBC8"
          target="_blank"
          className="text-gray-600 hover:underline whitespace-pre"
        >
          Feedback / Bug reports
        </a>
        &nbsp; &nbsp;
        <ThemeToggle />
      </div>
      <RouteLinks routes={menuRoutes} />
      {noHost && host.kind === "known" && (
        <HostNotice
          status={host.status}
          onGradioPage={isGradioPath(router.asPath.split("?")[0])}
        />
      )}
    </>
  );
};
