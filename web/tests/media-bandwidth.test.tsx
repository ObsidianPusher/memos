import { cleanup, render } from "@testing-library/react";
import React from "react";
import { afterEach, expect, it } from "vitest";
import MemoResourceListView from "@/components/MemoResourceListView";
import { Resource } from "@/types/proto/api/v1/resource_service";

afterEach(cleanup);

it("does not preload video or audio bytes before playback", () => {
  const view = render(
    <MemoResourceListView
      resources={[
        Resource.fromPartial({ name: "resources/video", filename: "video.mp4", type: "video/mp4" }),
        Resource.fromPartial({ name: "resources/audio", filename: "audio.mp3", type: "audio/mpeg" }),
      ]}
    />,
  );
  for (const media of view.container.querySelectorAll<HTMLMediaElement>("video, audio")) {
    expect(media.preload).toBe("none");
    expect(media.controls).toBe(true);
    expect(media.getAttribute("src")).toContain("/file/resources/");
  }
});

it("retains media elements and playback position when the memo resource list refreshes", () => {
  const video = Resource.fromPartial({ name: "resources/video", filename: "video.mp4", type: "video/mp4" });
  const view = render(<MemoResourceListView resources={[video]} />);
  const element = view.container.querySelector("video")!;
  element.currentTime = 15;
  view.rerender(<MemoResourceListView resources={[Resource.fromPartial(video)]} />);
  expect(view.container.querySelector("video")).toBe(element);
  expect(view.container.querySelector("video")!.currentTime).toBe(15);
});

it("starts media requests only when playback is requested in Chromium", async () => {
  const { chromium } = await import("playwright");
  const video = Resource.fromPartial({ name: "resources/video", filename: "video.mp4", type: "video/mp4" });
  const audio = Resource.fromPartial({ name: "resources/audio", filename: "audio.mp3", type: "audio/mpeg" });
  const view = render(<MemoResourceListView resources={[video, audio]} />);
  const html = view.container.innerHTML;
  const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH });
  try {
    const page = await browser.newPage();
    const requests: string[] = [];
    await page.route("**/file/resources/**", async (route) => {
      requests.push(route.request().url());
      await route.fulfill({ status: 200, contentType: "application/octet-stream", body: "media fixture" });
    });
    await page.setContent(html);
    await page.waitForLoadState("networkidle");
    expect(requests).toEqual([]);
    for (const selector of ["video", "audio"]) {
      await page.locator(selector).evaluate((el) => {
        void (el as HTMLMediaElement).play().catch(() => {});
      });
    }
    await expect.poll(() => requests.length).toBe(2);
  } finally {
    await browser.close();
  }
}, 30000);
