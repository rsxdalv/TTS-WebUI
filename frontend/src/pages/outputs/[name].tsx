import React from "react";
import { useRouter } from "@/shims/router";

import { HistoryCard } from "../../components/CardBig";
import { GenerationRaw } from "../../types/Generation";
import { Template } from "../../components/Template";
import { useGoData } from "../../data/useGoData";

// The empty list must be the same array every time, or the effect of
// useGoData would run on every render.
const NO_OUTPUTS: GenerationRaw[] = [];

export default function Home() {
  const { query } = useRouter();
  const name = typeof query.name === "string" ? query.name : "";
  const isFavorites = name === "favorites";
  // The list comes from the Go data endpoint; the empty list shows until it
  // answers (and if the request fails).
  const outputs = useGoData<GenerationRaw[]>(
    `/api/go-data/outputs/${encodeURIComponent(name)}`,
    NO_OUTPUTS
  );
  return (
    <Template title={isFavorites ? "Favorites" : "History"}>
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
        {outputs.map((generation) => (
          <HistoryCard
            key={generation.filename}
            metadata={generation}
            isFavorite={isFavorites}
          />
        ))}
      </div>
    </Template>
  );
}
