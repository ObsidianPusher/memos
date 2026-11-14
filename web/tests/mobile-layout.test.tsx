import { render, cleanup, act, waitFor, fireEvent } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { chromium } from "playwright";
import React from "react";
import { MemoryRouter } from "react-router-dom";
import { Route, Routes } from "react-router-dom";
import { expect, it, vi } from "vitest";
import showChangeMemberPasswordDialog from "@/components/ChangeMemberPasswordDialog";
import showCreateAccessTokenDialog from "@/components/CreateAccessTokenDialog";
import showCreateIdentityProviderDialog from "@/components/CreateIdentityProviderDialog";
import showCreateUserDialog from "@/components/CreateUserDialog";
import showCreateWebhookDialog from "@/components/CreateWebhookDialog";
import MathContent from "@/components/MemoContent/Math";
import Table from "@/components/MemoContent/Table";
import showUpdateAccountDialog from "@/components/UpdateAccountDialog";
import showUpdateCustomizedProfileDialog from "@/components/UpdateCustomizedProfileDialog";
import RootLayout from "@/layouts/RootLayout";
import EmotionPage from "@/pages/Emotion";
import { workspaceStore, userStore } from "@/store/v2";
import { Node, NodeType } from "@/types/proto/api/v1/markdown_service";
import { Emotion } from "@/types/proto/api/v1/memo_service";
import { User } from "@/types/proto/api/v1/user_service";
import postcss from "postcss";
import tailwindcss from "tailwindcss";

vi.mock("@/hooks/useEmotionMemos", () => ({
  useEmotionMemos: () => ({
    currentUser: { name: "users/1" },
    isLoading: false,
    error: null,
    report: {
      total: 2,
      averageScore: 1.5,
      dominantEmotion: Emotion.BAD,
      series: [{ labelLines: ["11/07", "11/13"], value: 1.5, count: 2, dominantEmotion: Emotion.BAD }],
      breakdown: [{ emotion: Emotion.BAD, count: 1, ratio: 50 }],
    },
  }),
}));

it("keeps report cards inside phone and tablet viewports while allowing the chart to scroll locally", async () => {
  workspaceStore.state.setPartial({ isInitialized: true });
  userStore.state.setPartial({ currentUser: "users/1", userMapByName: { "users/1": User.fromPartial({ name: "users/1" }) } });
  const require = createRequire(import.meta.url);
  const css = await postcss([tailwindcss(require("../tailwind.config.js"))]).process("@tailwind base; @tailwind utilities;", {
    from: require.resolve("../tailwind.config.js"),
  });
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
  try {
    for (const [width, fontSize] of [
      [320, 16],
      [320, 20],
      [390, 20],
      [768, 16],
      [1280, 20],
    ]) {
      Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
      const view = render(
        <MemoryRouter>
          <Routes>
            <Route element={<RootLayout />}>
              <Route index element={<EmotionPage />} />
            </Route>
          </Routes>
        </MemoryRouter>,
      );
      fireEvent.click(view.getByRole("button", { name: /180/ }));
      const html = view.container.innerHTML;
      cleanup();
      const page = await browser.newPage({ viewport: { width, height: 844 } });
      await page.setContent(`<html><head><style>${css.css} html { font-size: ${fontSize}px; }</style></head><body>${html}</body></html>`);
      for (const button of await page.locator('button[aria-pressed]').all()) {
        const box = await button.boundingBox();
        expect(box!.x + box!.width, `report control at ${width}px`).toBeLessThanOrEqual(width);
      }
      const chartScroll = await page
        .locator("svg")
        .last()
        .evaluate((el) => {
          const scroll = el.parentElement!;
          const before = scroll.scrollLeft;
          scroll.scrollLeft = 100;
          return { width: scroll.clientWidth, content: scroll.scrollWidth, moved: scroll.scrollLeft > before };
        });
      if (width < 768) expect(chartScroll.moved, `local chart scrolling at ${width}px`).toBe(true);
      const bounds = await page
        .locator("svg")
        .last()
        .evaluate((el) => {
          const card = el.closest("main")!;
          const row = document.querySelector("aside")!;
          return {
            cardRight: card.getBoundingClientRect().right,
            rowRight: row.getBoundingClientRect().right,
            scrollWidth: document.documentElement.scrollWidth,
          };
        });
      expect(bounds.cardRight, `chart card at ${width}px`).toBeLessThanOrEqual(width);
      expect(bounds.rowRight, `legend at ${width}px`).toBeLessThanOrEqual(width);
      expect(bounds.scrollWidth, `page at ${width}px`).toBeLessThanOrEqual(width);
      await page.close();
    }
  } finally {
    await browser.close();
  }
}, 30000);

it("keeps account dialog fields within a narrow mobile viewport", async () => {
  window.matchMedia = () => ({ matches: false, addListener() {}, removeListener() {} }) as unknown as MediaQueryList;
  act(() => showCreateUserDialog());
  const html = document.body.innerHTML;
  const require = createRequire(import.meta.url);
  const css = await postcss([tailwindcss(require("../tailwind.config.js"))]).process("@tailwind base; @tailwind utilities;", {
    from: require.resolve("../tailwind.config.js"),
  });
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
  try {
    const page = await browser.newPage({ viewport: { width: 320, height: 844 } });
    await page.setContent(`<style>${css.css} html {font-size:20px;}</style>${html}`);
    const bounds = await page.locator(".create-user-dialog > div").boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(16);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(304);
  } finally {
    await browser.close();
    act(() => document.body.dispatchEvent(new KeyboardEvent("keydown", { code: "Escape" })));
  }
}, 30000);

it("contains wide memo tables in a local horizontal scroller", async () => {
  const view = render(
    <div className="w-full word-break">
      <Table
        index="0"
        rows={[]}
        header={Array.from({ length: 16 }, () => Node.fromPartial({ type: NodeType.TEXT, textNode: { content: "Column" } }))}
      />
    </div>,
  );
  const html = view.container.innerHTML;
  cleanup();
  const require = createRequire(import.meta.url);
  const css = await postcss([tailwindcss(require("../tailwind.config.js"))]).process("@tailwind base; @tailwind utilities;", {
    from: require.resolve("../tailwind.config.js"),
  });
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
  try {
    const page = await browser.newPage({ viewport: { width: 320, height: 844 } });
    await page.setContent(`<style>${css.css} html {font-size:20px;}</style>${html}`);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
    expect(
      await page.locator("table").evaluate((el) => {
        const p = el.parentElement!;
        p.scrollLeft = 100;
        return p.scrollLeft;
      }),
    ).toBeGreaterThan(0);
  } finally {
    await browser.close();
  }
}, 30000);

for (const [name, showDialog] of [
  ["change-member-password-dialog", () => showChangeMemberPasswordDialog(User.fromPartial({ name: "users/1" }))],
  ["update-account-dialog", () => showUpdateAccountDialog()],
  ["create-access-token-dialog", () => showCreateAccessTokenDialog(() => {})],
  ["create-webhook-dialog", () => showCreateWebhookDialog(() => {})],
  ["create-identity-provider-dialog", () => showCreateIdentityProviderDialog()],
  ["update-customized-profile-dialog", () => showUpdateCustomizedProfileDialog()],
] as const) {
  it(`keeps ${name} fields and actions visible on a narrow phone`, async () => {
    window.matchMedia = () => ({ matches: false, addListener() {}, removeListener() {} }) as unknown as MediaQueryList;
    userStore.state.setPartial({ currentUser: "users/1", userMapByName: { "users/1": User.fromPartial({ name: "users/1" }) } });
    act(showDialog);
    const html = document.body.innerHTML;
    const require = createRequire(import.meta.url);
    const css = await postcss([tailwindcss(require("../tailwind.config.js"))]).process("@tailwind base; @tailwind utilities;", {
      from: require.resolve("../tailwind.config.js"),
    });
    const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
    try {
      const page = await browser.newPage({ viewport: { width: 320, height: 844 } });
      await page.setContent(`<style>${css.css} html {font-size:20px;}</style>${html}`);
      const bounds = await page.locator(`.${name} input, .${name} textarea, .${name} button`).evaluateAll((elements) =>
        elements
          .filter((el) => el.getBoundingClientRect().width > 0)
          .map((el) => ({
            tag: el.tagName,
            left: el.getBoundingClientRect().left,
            right: el.getBoundingClientRect().right,
          })),
      );
      expect(bounds.length).toBeGreaterThan(0);
      for (const field of bounds) {
        expect(field.left, `${field.tag} left`).toBeGreaterThanOrEqual(16);
        expect(field.right, `${field.tag} right`).toBeLessThanOrEqual(304);
      }
    } finally {
      await browser.close();
      act(() => document.body.dispatchEvent(new KeyboardEvent("keydown", { code: "Escape" })));
    }
  }, 30000);
}

it("keeps wide block equations in a local horizontal scroller", async () => {
  const view = render(
    <div className="w-full word-break">
      <MathContent block content="a+b+c+d+e+f+g+h+i+j+k+l+m+n+o+p=q" />
    </div>,
  );
  await waitFor(() => expect(view.container.querySelector(".katex")).not.toBeNull());
  const html = view.container.innerHTML;
  cleanup();
  const require = createRequire(import.meta.url);
  const css = await postcss([tailwindcss(require("../tailwind.config.js"))]).process("@tailwind base; @tailwind utilities;", {
    from: require.resolve("../tailwind.config.js"),
  });
  const katexCss = readFileSync(require.resolve("katex/dist/katex.min.css"), "utf8");
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
  try {
    const page = await browser.newPage({ viewport: { width: 320, height: 844 } });
    await page.setContent(`<style>${css.css} ${katexCss} html {font-size:20px;}</style>${html}`);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
    expect(
      await page.locator(".katex-display").evaluate((el) => {
        for (let parent = el.parentElement; parent; parent = parent.parentElement) {
          parent.scrollLeft = 100;
          if (parent.scrollLeft > 0 && getComputedStyle(parent).overflowX === "auto") return true;
        }
        return false;
      }),
    ).toBe(true);
  } finally {
    await browser.close();
  }
}, 30000);
