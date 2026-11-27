// Notice: the line an install on the old env sees on every page.
import { createMemo, Loading, Show } from "solid-js";
import { client } from "./client";

interface NoticeAnswer {
  legacy: boolean;
  text?: string;
  url?: string;
}

export function Notice() {
  // One GET on mount. Any failure reads as "no notice", never as an error.
  const notice = createMemo(() =>
    client("/api/app/notice")
      .then((r) => (r.ok ? (r.json() as Promise<NoticeAnswer>) : null))
      .catch(() => null),
  );
  const shown = createMemo(() => {
    const n = notice();
    return n?.legacy && n.text ? n : null;
  });

  return (
    <Loading fallback={null}>
      <Show when={shown()}>
        {(n) => (
          <div class="mb-6 p-4 rounded-lg bg-amber-50 dark:bg-amber-950/40 text-amber-900 dark:text-amber-200 text-sm">
            {n().text}{" "}
            <Show when={n().url}>
              {(url) => (
                <a href={url()} target="_blank" rel="noreferrer" class="underline">
                  Migration guide
                </a>
              )}
            </Show>
          </div>
        )}
      </Show>
    </Loading>
  );
}
