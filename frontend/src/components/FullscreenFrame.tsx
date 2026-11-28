import React from "react";
import { Button } from "./ui/button";
import { useTheme } from "@/lib/theme";

/**
 * An iframe with a Fullscreen button. Fullscreen covers the window; "Back"
 * (or Esc, when this page has focus) returns. It is one element whose classes
 * change, so the frame stays mounted and the page inside keeps its state.
 *
 * padded: the embedded page has no padding of its own (a single Gradio tab or
 * an extension page), so the container pads it on the left.
 */
export const FullscreenFrame = ({
  title,
  src,
  padded,
  full: controlledFull,
  onFullChange,
  hideToolbar,
}: {
  title: string;
  src: string;
  padded?: boolean;
  /** Controlled: the page owns the state and puts its own Fullscreen button elsewhere. */
  full?: boolean;
  onFullChange?: (full: boolean) => void;
  /** No toolbar row outside fullscreen (the page has its own Fullscreen button). */
  hideToolbar?: boolean;
}) => {
  const [ownFull, setOwnFull] = React.useState(false);
  const full = controlledFull ?? ownFull;
  const setFull = (v: boolean) => (onFullChange ? onFullChange(v) : setOwnFull(v));
  const { theme } = useTheme();
  // Gradio takes its theme from this query parameter, so the frame follows the toggle, not the OS.
  const themedSrc = src + (src.includes("?") ? "&" : "?") + "__theme=" + theme;

  React.useEffect(() => {
    if (!full) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFull(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [full]);

  // The container carries the colour of the embedded page (white / #0f0f0f; the frontend has no dark class, so it follows the OS scheme like the embedded page)
  // and, for pages embedded without padding of their own, a left pad.
  const bg = "bg-white dark:bg-[#0f0f0f]";
  return (
    <div
      className={
        full
          ? `fixed inset-0 z-[100] flex flex-col ${bg}`
          : `flex w-full flex-col ${bg}`
      }
    >
      {(full || !hideToolbar) && (
        <div className={full ? "flex items-center gap-3 border-b p-2" : "flex justify-end p-2"}>
          {full && <span className="font-medium">{title}</span>}
          <Button
            variant="outline"
            size="sm"
            className={full ? "ml-auto" : undefined}
            onClick={() => setFull(!full)}
          >
            {full ? "← Back (Esc)" : "Fullscreen"}
          </Button>
        </div>
      )}
      <div className={`flex ${full ? "min-h-0 flex-1" : ""} ${padded ? "pl-4" : ""}`}>
        <iframe
          title={title}
          src={themedSrc}
          className={full ? "h-full w-full flex-1" : "w-full"}
          style={full ? undefined : { height: "85vh" }}
        />
      </div>
    </div>
  );
};
