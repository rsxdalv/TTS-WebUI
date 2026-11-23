/**
 * A `next/head` compatible shim.
 *
 * Next collects the `<title>` and `<meta>` of every `<Head>` on the page and
 * puts them in the document head. This shim does the same: the children are
 * read for a title and for meta elements, and they are applied to
 * `document.head` while the component is mounted.
 *
 * Next keeps the title of the page that is on screen, so the title of a page
 * that is left is replaced by the title of the page that comes, and a page
 * without a title has none.
 */
import React, { useEffect } from "react";

type HeadChild = React.ReactElement<{
  children?: React.ReactNode;
  name?: string;
  property?: string;
  content?: string;
  charSet?: string;
}>;

/** The titles of the mounted Head components, in the order they mounted. */
const activeTitles: string[] = [];

const applyTitle = () => {
  document.title = activeTitles.length
    ? activeTitles[activeTitles.length - 1]
    : "";
};

const collect = (children: React.ReactNode) => {
  const titles: string[] = [];
  const metas: HeadChild[] = [];
  React.Children.forEach(children, (child) => {
    if (!React.isValidElement(child)) return;
    const element = child as HeadChild;
    if (element.type === "title") {
      titles.push(String(element.props.children ?? ""));
      return;
    }
    if (element.type === "meta") {
      metas.push(element);
    }
  });
  return { titles, metas };
};

const Head = ({ children }: { children?: React.ReactNode }) => {
  const { titles, metas } = collect(children);
  const title = titles[titles.length - 1];

  useEffect(() => {
    if (title === undefined) return;
    activeTitles.push(title);
    applyTitle();
    return () => {
      const index = activeTitles.indexOf(title);
      if (index >= 0) activeTitles.splice(index, 1);
      applyTitle();
    };
  }, [title]);

  useEffect(() => {
    const created: Element[] = [];
    for (const meta of metas) {
      const element = document.createElement("meta");
      for (const [key, value] of Object.entries(meta.props)) {
        if (key === "children" || value === undefined) continue;
        element.setAttribute(key === "charSet" ? "charset" : key, String(value));
      }
      document.head.appendChild(element);
      created.push(element);
    }
    return () => {
      for (const element of created) element.remove();
    };
    // The metas are recreated when the head changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [metas.length]);

  return null;
};

export default Head;
