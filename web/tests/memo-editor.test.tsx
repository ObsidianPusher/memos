import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import MemoEditor from "@/components/MemoEditor";
import { Memo, Visibility } from "@/types/proto/api/v1/memo_service";

const { memoStore, toast } = vi.hoisted(() => ({
  memoStore: { createMemo: vi.fn(), updateMemo: vi.fn(), getOrFetchMemoByName: vi.fn() },
  toast: { error: vi.fn(), success: vi.fn() },
}));
vi.mock("@/store/v2", () => ({
  memoStore,
  resourceStore: {},
  userStore: { state: { userSetting: { memoVisibility: "PRIVATE" } } },
  workspaceStore: { state: { memoRelatedSetting: {} } },
}));
vi.mock("@/grpcweb", () => ({ memoServiceClient: {} }));
vi.mock("@/hooks/useCurrentUser", () => ({ default: () => ({ name: "users/1" }) }));
vi.mock("@/utils/i18n", () => ({ useTranslate: () => (key: string) => key }));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ i18n: { language: "en" } }) }));
vi.mock("react-hot-toast", () => ({ toast }));
vi.mock("@/components/MemoEditor/Editor/TagSuggestions", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/TagSelector", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/MarkdownMenu", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/UploadResourceButton", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/AddMemoRelationPopover", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/VisibilitySelector", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ActionButton/EmotionSelector", () => ({ default: () => null }));
vi.mock("@/components/MemoEditor/ResourceListView", () => ({
  default: ({ resourceList }: { resourceList: unknown[] }) => <span>attachments:{resourceList.length}</span>,
}));
vi.mock("@/components/MemoEditor/RelationListView", () => ({ default: () => null }));
vi.mock("@/components/DateTimeInput", () => ({ default: () => null }));

beforeEach(() => {
  vi.resetAllMocks();
  localStorage.clear();
  vi.spyOn(console, "error").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function saveButton() {
  return screen.getByRole("button", { name: "editor.save" }) as HTMLButtonElement;
}
function editor() {
  return screen.getByRole("textbox") as HTMLTextAreaElement;
}

describe("memo save recovery", () => {
  it("retains the cached draft on failure and clears it after a successful retry", async () => {
    memoStore.createMemo.mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ name: "memos/saved" });
    const onConfirm = vi.fn();
    render(<MemoEditor cacheKey="draft" onConfirm={onConfirm} />);
    fireEvent.input(editor(), { target: { value: "keep this draft" } });
    fireEvent.click(saveButton());
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(editor().value).toBe("keep this draft");
    expect(JSON.parse(localStorage.getItem("users/1-draft")!)).toBe("keep this draft");
    expect(saveButton().disabled).toBe(false);
    fireEvent.click(saveButton());
    await waitFor(() => expect(onConfirm).toHaveBeenCalledWith("memos/saved"));
    expect(editor().value).toBe("");
    expect(localStorage.getItem("users/1-draft")).toBeNull();
  });

  it("keeps attachments when an update fails", async () => {
    memoStore.getOrFetchMemoByName.mockResolvedValue(
      Memo.fromPartial({
        name: "memos/existing",
        content: "original",
        visibility: Visibility.PRIVATE,
        resources: [{ name: "resources/attached" }],
      }),
    );
    memoStore.updateMemo.mockRejectedValue(new Error("offline"));
    render(<MemoEditor memoName="memos/existing" />);
    await waitFor(() => expect(editor().value).toBe("original"));
    fireEvent.input(editor(), { target: { value: "changed" } });
    fireEvent.click(saveButton());
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(screen.getByText("attachments:1")).toBeTruthy();
    expect(editor().value).toBe("changed");
    expect(saveButton().disabled).toBe(false);
  });

  it("releases the save button when the memo has no changes", async () => {
    memoStore.getOrFetchMemoByName.mockResolvedValue(
      Memo.fromPartial({ name: "memos/existing", content: "original", visibility: Visibility.PRIVATE }),
    );
    render(<MemoEditor memoName="memos/existing" />);
    await waitFor(() => expect(editor().value).toBe("original"));
    fireEvent.click(saveButton());
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("No changes detected"));
    expect(saveButton().disabled).toBe(false);
    expect(memoStore.updateMemo).not.toHaveBeenCalled();
  });
});
