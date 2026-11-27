import React from "react";
import Link from "@/shims/link";
import { ExternalLinkIcon, SendHorizonalIcon } from "lucide-react";
import { isGradioPath, useGradioAvailable } from "../functions/hostStatus";

export const ProjectCard = ({
  title, description, href, projectLink, target,
}: {
  title: string;
  description: string;
  href: string;
  projectLink?: string;
  target?: string;
}) => {
  const gradioAvailable = useGradioAvailable();
  const needsHost = !gradioAvailable && isGradioPath(href);
  return (
  <div className="flex flex-row justify-between gap-x-2 items-start cell">
    <h3 className="text-lg font-medium w-24 block flex-shrink-0 text-left">
      {title}
    </h3>
    <p className="text-base text-left">{description}</p>
    {needsHost ? (
      <span className="cell text-gray-400 flex items-center">
        Needs the Gradio host
      </span>
    ) : (
      <Link
        href={href}
        className="cell hover:bg-gray-200 flex items-center"
        target={target}
      >
        Run <SendHorizonalIcon className="ml-2 h-5 w-5 flex-shrink-0" />
      </Link>
    )}
    {projectLink && (
      <Link
        href={projectLink}
        className="cell hover:bg-gray-200 flex items-center"
        target="_blank"
      >
        Github
        <ExternalLinkIcon className="ml-2 h-5 w-5 flex-shrink-0" />
      </Link>
    )}
  </div>
  );
};
