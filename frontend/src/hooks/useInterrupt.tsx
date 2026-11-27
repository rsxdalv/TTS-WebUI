import { useRef } from "react";

export const useInterrupt = () => {
  const interrupted = useRef(false);

  const resetInterrupt = (callback: () => Promise<void>) => async () => {
    interrupted.current = false;
    await callback();
    interrupted.current = false;
  };

  const interrupt = () => (interrupted.current = true);

  return { interrupted, resetInterrupt, interrupt };
};
