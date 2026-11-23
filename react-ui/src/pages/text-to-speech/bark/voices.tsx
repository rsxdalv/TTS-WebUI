import { Inter } from "next/font/google";
import React from "react";

import { getNpzDataSimpleVoices } from "../../../data/getVoicesData";
import { Template } from "../../../components/Template";
import { CardVoiceNpz } from "../../../components/CardBig";
import { NPZ } from "../../../types/NPZ";
import { useGoData } from "../../../data/useGoData";

export const inter = Inter({ subsets: ["latin"] });

interface Props {
  // The server build passes the data (getStaticProps); the browser build gets
  // it from the API, so the props are optional.
  outputs?: NPZ[];
}

// The browser build has no props: the data comes from the API. The empty
// list must be the same array every time, or the effect of useGoData would
// run on every render.
const NO_OUTPUTS: NPZ[] = [];

export default function Voices({ outputs: staticOutputs = NO_OUTPUTS }: Props) {
  // With the Go proxy's data-bark-voices claim on, the list comes from Go;
  // otherwise (and if the request fails) the getStaticProps value is shown.
  const outputs = useGoData<NPZ[]>(
    "data-bark-voices",
    "/api/go-data/bark-voices",
    staticOutputs
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

export const getStaticProps = async ({}): Promise<{ props: Props }> => {
  // The static export gets its data from the Go endpoint (see useGoData).
  const outputs: NPZ[] =
    process.env.TTS_STATIC_EXPORT === "1" ? [] : await getNpzDataSimpleVoices();
  return {
    props: {
      outputs,
    },
  };
};
