import React from "react";

import { CardEmpty, CardGeneration } from "../../components/CardBig";
import { GenerationRaw } from "../../types/Generation";
import { Template } from "../../components/Template";

// The browser build has no props. The empty list must be the same array
// every time, or an effect would run on every render.
const NO_GENERATIONS: GenerationRaw[] = [];

export default function Home({
  generations = NO_GENERATIONS,
}: {
  // The server build passes the data (getStaticProps); the browser build has
  // no props.
  generations?: GenerationRaw[];
}) {
  return (
    <Template title="Favorites">
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
        {generations.map((generation) => (
          <CardGeneration key={generation.date} metadata={generation} />
        ))}
        <CardEmpty
          title="Add a new generation"
          link="https://github.com/rsxdalv/bark-speaker-directory/pull/7"
        />
      </div>{" "}
    </Template>
  );
}

export const getStaticProps = async () => {
  // const generations: GenerationRaw[] = await getOggData();
  const generations: GenerationRaw[] = [];
  return {
    props: {
      generations: generations,
    },
  };
};
