import React, { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useRouter } from "next/router";

import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";

// In the static export (npm run build:static) there is one page for every
// extension name: Go serves extensions/_shell.html for /extensions/<any name>,
// and the name is then read from the URL. The server build never renders it.
const SHELL = "_shell";

const nameFromPath = (path: string) => {
  const m = /^\/extensions\/([^/?#]+)/.exec(path);
  if (!m) return "";
  try {
    return decodeURIComponent(m[1]);
  } catch {
    return m[1];
  }
};

export default function Home({ name: propName }: { name?: string }) {
  const { asPath } = useRouter();
  const [urlName, setUrlName] = useState("");
  // The server build and the pre-rendered extension get their name as a prop.
  // The shell does not know it until it runs in the browser (after hydration,
  // so the first render matches the exported HTML).
  useEffect(() => {
    if (!propName || propName === SHELL) setUrlName(nameFromPath(asPath));
  }, [propName, asPath]);
  const name = !propName || propName === SHELL ? urlName : propName;
  const ExtensionPage = useMemo(
    () =>
      dynamic(
        () =>
          import("../../extensions/link").then((m) => m[name] || (() => null)),
        {
          ssr: false,
        }
      ),
    [name]
  );
  return (
    <Template title={name}>
      <ExtensionPage {...({ Button } as any)} />
    </Template>
  );
}

export const getStaticProps = async ({ params: { name } }) => {
  return { props: { name } };
};

export const getStaticPaths = async () => {
  const isStaticExport = process.env.TTS_STATIC_EXPORT === "1";
  return {
    paths: isStaticExport
      ? ["/extensions/extension_xtts_simple", `/extensions/${SHELL}`]
      : ["/extensions/extension_xtts_simple"],
    // An export has no server to render other names on demand.
    fallback: isStaticExport ? false : "blocking",
  };
};
