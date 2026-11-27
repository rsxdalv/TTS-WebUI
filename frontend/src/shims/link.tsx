/**
 * A `Link` on top of react-router's `Link`.
 *
 * The two call sites (src/components/Header.tsx, src/components/ProjectCard.tsx)
 * use `href`, `className` and `target`, so the props are passed straight
 * through. External links (an absolute URL) are plain anchors.
 */
import React from "react";
import { Link as RouterLink, type LinkProps } from "react-router-dom";

const isExternal = (href: string) => /^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(href);

const Link = ({
  href,
  children,
  ...rest
}: { href: string; children?: React.ReactNode } & Omit<
  React.AnchorHTMLAttributes<HTMLAnchorElement>,
  "href"
>) => {
  if (isExternal(href)) {
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  }
  return (
    <RouterLink {...(rest as Omit<LinkProps, "to">)} to={href}>
      {children}
    </RouterLink>
  );
};

export default Link;
