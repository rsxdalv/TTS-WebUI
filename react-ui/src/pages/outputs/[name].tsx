import React from "react";
import { useRouter } from "next/router";

import { HistoryCard } from "../../components/CardBig";
import { getDataFromJSON } from "../../data/getVoicesData";
import { GenerationRaw } from "../../types/Generation";
import { Template } from "../../components/Template";
import { useGoData } from "../../data/useGoData";

interface Props {
  outputs: GenerationRaw[];
  isFavorites?: boolean;
}

export default function Home({
  outputs: staticOutputs,
  isFavorites = false,
}: Props) {
  const { query } = useRouter();
  const name = typeof query.name === "string" ? query.name : "";
  // With the Go proxy's data-outputs claim on, the list comes from Go;
  // otherwise (and if the request fails) the getStaticProps value is shown.
  const outputs = useGoData<GenerationRaw[]>(
    "data-outputs",
    `/api/go-data/outputs/${encodeURIComponent(name)}`,
    staticOutputs
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

// The static export (npm run build:static) must not bake the machine's
// generations into the page: the data comes from the Go endpoint instead.
const isStaticExport = process.env.TTS_STATIC_EXPORT === "1";

export const getStaticProps = async ({ params: { name } }) => {
  const outputs: GenerationRaw[] = isStaticExport
    ? []
    : await getDataFromJSON(name);
  return {
    props: {
      outputs,
      isFavorites: name === "favorites",
    },
  } as { props: Props };
};

export const getStaticPaths = async () => {
  return {
    paths: [
      "/outputs/outputs",
      "/outputs/favorites",
      // "/history/collections",
    ],
    // An export has no server to render other names on demand.
    fallback: isStaticExport ? false : "blocking",
  };
};
