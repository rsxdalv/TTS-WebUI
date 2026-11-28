import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SimpleMarkdown } from "./SimpleMarkdown";

describe("SimpleMarkdown", () => {
  it("renders bold, headings and lists without the markers", () => {
    const html = renderToStaticMarkup(
      <SimpleMarkdown text={"**Total Log Files:** 3\n### ❌ Errors Found:\n- one\n\n# 📦 Pip Logs Summary"} />
    );
    expect(html).toContain("<strong>Total Log Files:</strong>");
    expect(html).not.toContain("**");
    expect(html).not.toContain("###");
    expect(html).toContain("❌ Errors Found:");
    expect(html).toContain("one");
    expect(html).toContain("📦 Pip Logs Summary");
  });
  it("leaves a lone ** alone", () => {
    expect(renderToStaticMarkup(<SimpleMarkdown text="a ** b" />)).toContain("a ** b");
  });
});
