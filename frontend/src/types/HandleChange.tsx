import React from "react";

export type HandleChangeEvent =
  | React.ChangeEvent<HTMLInputElement>
  | React.ChangeEvent<HTMLTextAreaElement>
  | React.ChangeEvent<HTMLSelectElement>
  | {
      target: {
        name: string;
        value: string | number | boolean | null | undefined;
        type?: string;
        checked?: boolean;
      };
    };

export type HandleChange = (event: HandleChangeEvent) => void;
