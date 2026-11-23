import { useEffect, useState } from "react";
import { dataComesFromApi } from "./env";

// True when the Go proxy says it answers this data claim. The proxy sets the
// cookie on page loads; with the proxy off nothing sets it (CONTRACTS.md section 7).
// A static export (npm run build:static) has no Next route to fall back to, so
// there the endpoint is always asked.
const claimIsOn = (claim: string) =>
  dataComesFromApi() ||
  (document.cookie
    .split("; ")
    .find((c) => c.startsWith("tts-go-claims="))
    ?.slice("tts-go-claims=".length)
    .split(".")
    .includes(claim) ?? false);

/**
 * Returns `initial` (the getStaticProps value) and, when the Go proxy answers
 * the claim, replaces it with the endpoint's response. A failed request leaves
 * `initial` on screen.
 */
export function useGoData<T>(claim: string, url: string, initial: T): T {
  const [data, setData] = useState(initial);
  useEffect(() => {
    setData(initial);
    if (!claimIsOn(claim)) return;
    let cancelled = false;
    fetch(url)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (!cancelled && d !== null) setData(d);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [claim, url, initial]);
  return data;
}
