// Notice: the line an install on the old env sees on every page.
import { useEffect, useState } from "react";
import { client } from "./client";

interface NoticeAnswer {
  legacy: boolean;
  text?: string;
  url?: string;
}

export function Notice() {
  const [n, setN] = useState<NoticeAnswer | null>(null);
  useEffect(() => {
    client("/api/app/notice")
      .then((r) => (r.ok ? r.json() : null))
      .then(setN)
      .catch(() => setN(null));
  }, []);
  if (!n?.legacy || !n.text) return null;
  return (
    <div className="mb-6 p-4 rounded-lg bg-amber-50 dark:bg-amber-950/40 text-amber-900 dark:text-amber-200 text-sm">
      {n.text}{" "}
      {n.url && (
        <a href={n.url} target="_blank" rel="noreferrer" className="underline">
          Migration guide
        </a>
      )}
    </div>
  );
}
