import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import React, { Profiler } from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import StorageSection from "@/components/Settings/StorageSection";
import WorkspaceSection from "@/components/Settings/WorkspaceSection";
import i18n from "@/i18n";
import en from "@/locales/en.json";
import { workspaceStore } from "@/store/v2";
import { WorkspaceSetting, WorkspaceStorageSetting_StorageType as StorageType } from "@/types/proto/api/v1/workspace_setting_service";

vi.mock("@/grpcweb", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/grpcweb")>();
  return {
    ...actual,
    identityProviderServiceClient: {
      ...actual.identityProviderServiceClient,
      listIdentityProviders: async () => ({ identityProviders: [] }),
    },
  };
});

beforeEach(async () => {
  workspaceStore.state.setPartial({ settings: [] });
  await i18n.init({ lng: "en", resources: { en: { translation: en } }, fallbackLng: "en" });
});
afterEach(cleanup);

function renderSettings(component: React.ReactNode) {
  let commits = 0;
  return render(
    <MemoryRouter>
      <Profiler
        id="settings"
        onRender={() => {
          // Stop a real render loop before it hangs the test process.
          if (++commits > 30) throw new Error("Settings did not settle after 30 commits");
        }}
      >
        {component}
      </Profiler>
    </MemoryRouter>,
  );
}

it("keeps storage selection and edits usable when no storage setting has been saved", () => {
  renderSettings(<StorageSection />);
  fireEvent.click(screen.getByRole("radio", { name: "Local file system" }));
  expect((screen.getByRole("radio", { name: "Local file system" }) as HTMLInputElement).checked).toBe(true);
  const path = screen.getByPlaceholderText("assets/{timestamp}_{filename}");
  fireEvent.change(path, { target: { value: "uploads/{filename}" } });
  expect((path as HTMLInputElement).value).toBe("uploads/{filename}");
  fireEvent.click(screen.getByRole("radio", { name: "S3" }));
  expect((screen.getByRole("radio", { name: "S3" }) as HTMLInputElement).checked).toBe(true);
});

it("keeps system settings editable when no general setting has been saved", async () => {
  renderSettings(<WorkspaceSection />);
  const style = screen.getAllByRole("textbox")[0];
  fireEvent.change(style, { target: { value: "body { color: red; }" } });
  await waitFor(() => expect((style as HTMLTextAreaElement).value).toBe("body { color: red; }"));
});

it("loads arriving storage settings and retains edits through unrelated store updates", () => {
  renderSettings(<StorageSection />);
  act(() =>
    workspaceStore.state.setPartial({
      settings: [
        WorkspaceSetting.fromPartial({
          name: "settings/STORAGE",
          storageSetting: { storageType: StorageType.LOCAL, uploadSizeLimitMb: 32, filepathTemplate: "files/{filename}" },
        }),
      ],
    }),
  );
  const path = screen.getByPlaceholderText("assets/{timestamp}_{filename}");
  expect((path as HTMLInputElement).value).toBe("files/{filename}");
  fireEvent.change(path, { target: { value: "edited/{filename}" } });
  act(() => workspaceStore.state.setPartial({ fontSize: 18 }));
  expect((path as HTMLInputElement).value).toBe("edited/{filename}");
});
