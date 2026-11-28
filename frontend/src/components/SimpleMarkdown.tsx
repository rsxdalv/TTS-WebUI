import React from "react";

/** **bold** and `code` inside one line. */
const inline = (text: string): React.ReactNode[] =>
  text
    .split(/(\*\*[^*]+\*\*|`[^`]+`)/g)
    .filter((part) => part !== "")
    .map((part, i) =>
      part.startsWith("**") && part.endsWith("**") && part.length > 4 ? (
        <strong key={i}>{part.slice(2, -2)}</strong>
      ) : part.startsWith("`") && part.endsWith("`") && part.length > 2 ? (
        <code key={i} className="font-mono">
          {part.slice(1, -1)}
        </code>
      ) : (
        <React.Fragment key={i}>{part}</React.Fragment>
      )
    );

const HEADINGS: Record<number, string> = {
  1: "text-lg font-bold mt-2",
  2: "text-base font-bold mt-2",
  3: "text-sm font-bold mt-2",
};

/**
 * The small part of Markdown the Go core's text endpoints use (headings,
 * **bold**, `code`, "- " lists, blank lines). Not a general renderer.
 */
export const SimpleMarkdown = ({
  text,
  className = "",
}: {
  text: string;
  className?: string;
}) => (
  <div className={`text-sm ${className}`}>
    {text.split("\n").map((line, i) => {
      const h = /^(#{1,3})\s+(.*)$/.exec(line);
      if (h) {
        return (
          <div key={i} className={HEADINGS[h[1].length]}>
            {inline(h[2])}
          </div>
        );
      }
      const li = /^\s*[-*]\s+(.*)$/.exec(line);
      if (li) {
        return (
          <div key={i} className="pl-4 -indent-3 before:content-['•_']">
            {inline(li[1])}
          </div>
        );
      }
      return line.trim() === "" ? (
        <div key={i} className="h-2" />
      ) : (
        <div key={i}>{inline(line)}</div>
      );
    })}
  </div>
);
