import React from "react";
import { useRouter } from "next/router";

import { HistoryCard } from "../../components/CardBig";
import { getDataFromJSON } from "../../data/getVoicesData";
import { GenerationRaw } from "../../types/Generation";
import { Template } from "../../components/Template";
import { useGoData } from "../../data/useGoData";

interface Props {
  // The server build passes the data (getStaticProps); the browser build gets
  // it from the API, so the props are optional.
  outputs?: GenerationRaw[];
  isFavorites?: boolean;
}

// The browser build has no props: the data comes from the API. The empty
// list must be the same array every time, or the effect of useGoData would
// run on every render.
const NO_OUTPUTS: GenerationRaw[] = [];

export default function Home({
  outputs: staticOutputs = NO_OUTPUTS,
  isFavorites: propIsFavorites,
}: Props) {
  const { query } = useRouter();
  const name = typeof query.name === "string" ? query.name : "";
  // The server build gets the flag from getStaticProps; the browser reads it
  // from the URL.
  const isFavorites = propIsFavorites ?? name === "favorites";
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
