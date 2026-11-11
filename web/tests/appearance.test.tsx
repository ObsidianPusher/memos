import { CssVarsProvider } from "@mui/joy";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { runInAction } from "mobx";
import React from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import App from "@/App";
import PreferencesSection from "@/components/Settings/PreferencesSection";
import i18n from "@/i18n";
import en from "@/locales/en.json";
import { userStore, workspaceStore } from "@/store/v2";
import theme from "@/theme";
import { User, UserSetting } from "@/types/proto/api/v1/user_service";

// Only replace network calls; selectors, stores, App and Joy are real.
vi.mock("@/grpcweb", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/grpcweb")>();
  return {
    ...actual,
    userServiceClient: {
      ...actual.userServiceClient,
      updateUserSetting: async ({ setting }: { setting: Partial<UserSetting> }) =>
        UserSetting.fromPartial({ locale: "en", appearance: "light", memoVisibility: "PRIVATE", ...setting }),
    },
    webhookServiceClient: { ...actual.webhookServiceClient, listWebhooks: async () => ({ webhooks: [] }) },
  };
});

let systemDark = false;
let media: MediaQueryList;
beforeEach(async () => {
  localStorage.clear();
  document.documentElement.className = "";
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.style.fontSize = "";
  systemDark = false;
  media = Object.assign(new EventTarget(), {
    get matches() {
      return systemDark;
    },
    media: "(prefers-color-scheme: dark)",
    onchange: null,
    addListener() {},
    removeListener() {},
  }) as unknown as MediaQueryList;
  Object.defineProperty(media, "matches", { get: () => systemDark });
  vi.stubGlobal("matchMedia", () => media);
  window.matchMedia = () => media;
  await i18n.init({ lng: "en", resources: { en: { translation: en } }, fallbackLng: "en" });
  runInAction(() => {
    workspaceStore.state.setPartial({ appearance: "light", locale: "en" });
    userStore.state.setPartial({
      currentUser: "users/1",
      userMapByName: { "users/1": User.fromPartial({ name: "users/1" }) },
      userSetting: UserSetting.fromPartial({ appearance: "light", locale: "en", memoVisibility: "PRIVATE" }),
    });
  });
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function renderPreferences() {
  return render(
    <CssVarsProvider theme={theme} defaultMode="light">
      <MemoryRouter>
        <Routes>
          <Route element={<App />}>
            <Route path="/" element={<PreferencesSection />} />
          </Route>
        </Routes>
      </MemoryRouter>
    </CssVarsProvider>,
  );
}
function changeSystemTheme(dark: boolean) {
  act(() => {
    systemDark = dark;
    const event = new Event("change");
    Object.defineProperty(event, "matches", { value: dark });
    media.dispatchEvent(event);
  });
}

it("saves Dracula through the existing user setting and applies dark surfaces", async () => {
  renderPreferences();
  fireEvent.click(screen.getAllByRole("combobox")[1]);
  fireEvent.click(await screen.findByRole("option", { name: "Dracula" }));
  await waitFor(() => expect(userStore.state.userSetting?.appearance).toBe("dracula"));
  expect(workspaceStore.state.appearance).toBe("dracula");
  await waitFor(() => expect(document.documentElement.dataset.theme).toBe("dracula"));
  expect(document.documentElement.classList.contains("dark")).toBe(true);
  changeSystemTheme(false);
  expect(document.documentElement.classList.contains("dark")).toBe(true);
});

it("keeps explicit light mode when the operating system switches to dark", async () => {
  renderPreferences();
  await waitFor(() => expect(document.documentElement.classList.contains("dark")).toBe(false));
  changeSystemTheme(true);
  await waitFor(() => expect(document.documentElement.classList.contains("dark")).toBe(false));
});

it("follows operating system changes only in system mode", async () => {
  renderPreferences();
  act(() => userStore.state.setPartial({ userSetting: UserSetting.fromPartial({ appearance: "system", locale: "en" }) }));
  changeSystemTheme(true);
  await waitFor(() => expect(document.documentElement.classList.contains("dark")).toBe(true));
  changeSystemTheme(false);
  await waitFor(() => expect(document.documentElement.classList.contains("dark")).toBe(false));
});

it("applies the selected font size and retains it across remounts", async () => {
  const view = renderPreferences();
  fireEvent.click(screen.getByRole("combobox", { name: "Font size (this browser)" }));
  fireEvent.click(await screen.findByRole("option", { name: "18px" }));
  await waitFor(() => expect(document.documentElement.style.fontSize).toBe("18px"));
  expect(localStorage.getItem("memos-font-size")).toBe("18");
  view.unmount();
  document.documentElement.style.fontSize = "";
  renderPreferences();
  await waitFor(() => expect(document.documentElement.style.fontSize).toBe("18px"));
});

it("keeps Dracula when the workspace state is refreshed", () => {
  workspaceStore.state.setPartial({ appearance: "dracula" });
  workspaceStore.state.setPartial({ locale: "en" });
  expect(workspaceStore.state.appearance).toBe("dracula");
});

it("restores a saved font size when the browser reloads the store", async () => {
  localStorage.setItem("memos-font-size", "20");
  vi.resetModules();
  const { default: reloaded } = await import("@/store/v2/workspace");
  expect(reloaded.state.fontSize).toBe(20);
});

it.each(["", "NaN", "999", "-1", "17"])("uses the default font size for invalid storage %j", async (value) => {
  localStorage.setItem("memos-font-size", value);
  vi.resetModules();
  const { default: reloaded } = await import("@/store/v2/workspace");
  expect(reloaded.state.fontSize).toBe(16);
});
