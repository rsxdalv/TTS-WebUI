import React from "react";
import { Button } from "./ui/button";

export const ResetButton = <T extends {}>({
  params,
  setParams,
  initialParams,
}: {
  params: T;
  setParams: React.Dispatch<React.SetStateAction<T>>;
  initialParams: T;
}) => (
  <Button
    variant="outline"
    onClick={() =>
      setParams({
        ...params,
        ...initialParams,
      })
    }
  >
    Reset Parameters
  </Button>
);
