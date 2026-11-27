import React, { useMemo } from "react";
import dynamic from "@/shims/dynamic";
import { useRouter } from "@/shims/router";

import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";

// One page serves every extension name: the name is read from the URL.
const nameFromPath = (path: string) => {
  const m = /^\/extensions\/([^/?#]+)/.exec(path);
  if (!m) return "";
  try {
    return decodeURIComponent(m[1]);
  } catch {
    return m[1];
  }
};

export default function Home() {
  const { asPath } = useRouter();
  const name = useMemo(() => nameFromPath(asPath), [asPath]);
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
