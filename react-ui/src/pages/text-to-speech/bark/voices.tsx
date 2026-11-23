import { Inter } from "@/shims/font";
import React from "react";

import { Template } from "../../../components/Template";
import { CardVoiceNpz } from "../../../components/CardBig";
import { NPZ } from "../../../types/NPZ";
import { useGoData } from "../../../data/useGoData";

export const inter = Inter({ subsets: ["latin"] });

// The empty list must be the same array every time, or the effect of
// useGoData would run on every render.
const NO_OUTPUTS: NPZ[] = [];

export default function Voices() {
  // The list comes from the Go data endpoint; the empty list shows until it
  // answers (and if the request fails).
  const outputs = useGoData<NPZ[]>(
    "/api/go-data/bark-voices",
    NO_OUTPUTS
  );
  return (
    <Template title="Voices">
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
        {outputs.map((npz) => (
          <CardVoiceNpz key={npz.filename} generation={npz} />
        ))}
      </div>
    </Template>
  );
}
