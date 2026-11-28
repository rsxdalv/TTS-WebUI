import React from "react";
import { Button } from "./ui/button";

/**
 * An iframe with a Fullscreen button. Fullscreen covers the window; "Back"
 * (or Esc, when this page has focus) returns. It is one element whose classes
 * change, so the frame stays mounted and the page inside keeps its state.
 *
 * padded: the embedded page has no padding of its own (a single Gradio tab is
 * embedded without it), so fullscreen puts it back around the frame.
 */
export const FullscreenFrame = ({
  title,
  src,
  padded,
}: {
  title: string;
  src: string;
  padded?: boolean;
}) => {
  const [full, setFull] = React.useState(false);

  React.useEffect(() => {
    if (!full) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFull(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [full]);

  return (
    <div
      className={
        full
          ? "fixed inset-0 z-[100] flex flex-col bg-white dark:bg-gray-950"
          : "flex w-full flex-col gap-2"
      }
    >
      <div className={full ? "flex items-center gap-3 border-b p-2" : "flex justify-end"}>
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
      <div className={full ? `flex min-h-0 flex-1 ${padded ? "p-4" : ""}` : "contents"}>
        <iframe
          title={title}
          src={src}
          className={full ? "h-full w-full flex-1" : "w-full border rounded"}
          style={full ? undefined : { height: "85vh" }}
        />
      </div>
    </div>
  );
};
