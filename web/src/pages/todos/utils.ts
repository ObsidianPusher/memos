import { Memo } from "@/types/proto/api/v1/memo_service";

export const ALL_TAG_FILTER = "__ALL__";
export const NO_TAG_FILTER = "__NO_TAG__";
const TASK_LINE_PATTERN = /^[-+*]\s+\[(?: |x|X)\]\s+/;

export const normalizeMemoTags = (memo: Memo) => {
  const uniqueTags = new Set<string>();
  for (const tag of memo.tags || []) {
    const normalizedTag = tag.trim();
    if (normalizedTag) {
      uniqueTags.add(normalizedTag);
    }
  }
  return Array.from(uniqueTags);
};

export const buildTodoMemoContent = (rawInput: string) => {
  const normalizedLines = rawInput
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
  return normalizedLines.map((line) => (TASK_LINE_PATTERN.test(line) ? line : `- [ ] ${line}`)).join("\n");
};
