import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import dayjs from "dayjs";
import React from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import EmotionPage from "@/pages/Emotion";
import { userStore, workspaceStore } from "@/store/v2";
import { User } from "@/types/proto/api/v1/user_service";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("offers 30, 90 and 180 days and requests the selected interval without changing aggregation", async () => {
  await i18n.changeLanguage("zh-Hans");
  userStore.state.setPartial({ currentUser: "users/1", userMapByName: { "users/1": User.fromPartial({ name: "users/1" }) } });
  workspaceStore.state.setPartial({ isInitialized: true });
  const urls: URL[] = [];
  vi.stubGlobal("fetch", async (input: string) => {
    urls.push(new URL(input, "http://localhost"));
    return new Response(JSON.stringify({ total: 0, averageScore: 0, dominantEmotion: "EMOTION_UNSPECIFIED", breakdown: [], series: [] }), {
      status: 200,
    });
  });
  const view = render(
    <MemoryRouter>
      <EmotionPage />
    </MemoryRouter>,
  );
  const thirty = view.getByRole("button", { name: "30 天" });
  expect(view.queryByRole("button", { name: /^7(?:D| 天)$/ })).toBeNull();
  expect(thirty.getAttribute("aria-pressed")).toBe("true");
  for (const days of [90, 180, 30]) {
    fireEvent.click(view.getByRole("button", { name: `${days} 天` }));
    await waitFor(() => {
      const url = urls.at(-1)!;
      expect(
        dayjs(url.searchParams.get("end_time"))
          .startOf("day")
          .diff(dayjs(url.searchParams.get("start_time")), "day"),
      ).toBe(days - 1);
      expect(url.searchParams.get("granularity")).toBe("week");
    });
    expect(view.getByRole("button", { name: `${days} 天` }).getAttribute("aria-pressed")).toBe("true");
  }
  fireEvent.click(view.getByRole("button", { name: "月" }));
  await waitFor(() => expect(urls.at(-1)!.searchParams.get("granularity")).toBe("month"));
  expect(view.getByRole("heading", { name: "情绪趋势" })).toBeTruthy();
  expect(view.getByText("情绪分布")).toBeTruthy();
  expect(view.queryByText("报表摘要")).toBeNull();
});
