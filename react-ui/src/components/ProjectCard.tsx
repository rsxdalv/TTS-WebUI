import React from "react";
import Link from "next/link";
import { ExternalLinkIcon, SendHorizonalIcon } from "lucide-react";

export const ProjectCard = ({
  title, description, href, projectLink, target,
}: {
  title: string;
  description: string;
  href: string;
  projectLink?: string;
  target?: string;
}) => (
  <div className="flex flex-row justify-between gap-x-2 items-start cell">
    <h3 className="text-lg font-medium w-24 block flex-shrink-0 text-left">
      {title}
    </h3>
    <p className="text-base text-left">{description}</p>
    <Link
      href={href}
      className="cell hover:bg-gray-200 flex items-center"
      target={target}
    >
      Run <SendHorizonalIcon className="ml-2 h-5 w-5 flex-shrink-0" />
    </Link>
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
