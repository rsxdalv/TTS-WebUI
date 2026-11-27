import React from "react";
import { HandleChangeEvent } from "../types/HandleChange";

export const parseFormChange =
  <T extends {}>(setter: React.Dispatch<React.SetStateAction<T>>) =>
  (event: HandleChangeEvent) => {
    const { name, value, type } = event.target;
    setter((x: T) => ({
      ...x,
      [name]:
        type === "number" || type === "range"
          ? Number(value)
          : type === "checkbox"
          ? (event.target as HTMLInputElement).checked // type assertion
          : value,
    }));
  };
