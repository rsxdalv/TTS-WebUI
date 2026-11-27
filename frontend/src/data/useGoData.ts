import { useEffect, useState } from "react";

/**
 * Returns `initial` until the endpoint answers, then the endpoint's response.
 * A failed request leaves `initial` on screen.
 */
export function useGoData<T>(url: string, initial: T): T {
  const [data, setData] = useState(initial);
  useEffect(() => {
    setData(initial);
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
  }, [url, initial]);
  return data;
}
