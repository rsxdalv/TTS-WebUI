import React from "react";
import { Button } from "./ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";
import { LauncherExtensionsLink } from "./LauncherExtensionsLink";
import type { Run } from "../functions/extensionOps";

/**
 * The dialog of an install or uninstall: the step, the live log and Close once
 * it is done. `run` is the running one (null closes the dialog); `view` is the
 * last run, shown while the dialog fades out.
 */
export const ExtensionRunDialog = ({
  run,
  view,
  onClose,
}: {
  run: Run | null;
  view: Run | null;
  onClose: () => void;
}) => {
  const logRef = React.useRef<HTMLPreElement>(null);
  React.useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [view?.lines.length]);

  return (
    <Dialog open={!!run} onOpenChange={(open) => !open && run?.done && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            {view?.op === "install" ? "Installing" : "Uninstalling"} {view?.name}
            {view?.progress ? ` (${view.progress})` : ""}
          </DialogTitle>
          <DialogDescription>
            {!view?.done
              ? view?.step || "Working. Installs can take several minutes."
              : view?.ok
                ? view?.proxy === "native"
                  ? "Done."
                  : "Done. Restart the app to load or unload it."
                : "It did not finish."}
          </DialogDescription>
        </DialogHeader>
        <pre
          ref={logRef}
          className="max-h-[50vh] overflow-auto rounded bg-gray-100 p-2 text-xs dark:bg-gray-900"
        >
          {view?.lines.join("\n")}
        </pre>
        {view?.needsGpu && (
          <p className="text-sm">
            This install needs a GPU choice first. Make it on the{" "}
            <LauncherExtensionsLink text="Extensions page of the launcher" />.
          </p>
        )}
        <DialogFooter>
          <Button disabled={!view?.done} onClick={onClose}>
            {view?.done ? "Close" : "Working…"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
