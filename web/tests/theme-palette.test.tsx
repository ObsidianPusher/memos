import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { afterEach, expect, it } from "vitest";
import postcss from "postcss";
import tailwindcss from "tailwindcss";

const require = createRequire(import.meta.url);
const config = require("../tailwind.config.js");
let style: HTMLStyleElement | undefined;
afterEach(() => {
  style?.remove();
  document.documentElement.removeAttribute("data-theme");
});

it("applies the Dracula palette only while the Dracula theme is selected", async () => {
  const input = postcss.parse(readFileSync(require.resolve("../src/css/tailwind.css"), "utf8"));
  input.walkAtRules("import", (rule) => {
    rule.remove();
  });
  const result = await postcss([
    tailwindcss({ ...config, content: [{ raw: '<div class="dark:bg-zinc-800 text-gray-300 bg-primary"></div>', extension: "html" }] }),
  ]).process(input, { from: undefined });
  style = document.createElement("style");
  style.textContent = result.css;
  document.head.appendChild(style);
  const root = document.documentElement;
  root.dataset.theme = "dracula";
  expect(getComputedStyle(root).getPropertyValue("--memos-zinc-800").trim()).toBe("40 42 54");
  expect(getComputedStyle(root).getPropertyValue("--joy-palette-background-surface").trim()).toBe("#282a36");
  expect(getComputedStyle(root).getPropertyValue("--memos-primary").trim()).toBe("189 147 249");
  root.dataset.theme = "light";
  expect(getComputedStyle(root).getPropertyValue("--memos-zinc-800").trim()).toBe("");
});
