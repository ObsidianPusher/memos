import { Button, Switch } from "@usememos/mui";
import dayjs from "dayjs";
import { DownloadIcon, LinkIcon, SearchIcon, SmilePlusIcon, TagIcon } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { listAllMemos } from "@/api/memo";
import useCurrentUser from "@/hooks/useCurrentUser";
import type { MemoFilter } from "@/store/v2/memoFilter";
import { Direction, State } from "@/types/proto/api/v1/common";
import { Emotion, Memo } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";
import { ANY_EMOTION_FILTER, EMOTION_OPTIONS, getEmotionSearchValues } from "@/utils/emotion";
import { getErrorMessage } from "@/utils/error";
import { useTranslate } from "@/utils/i18n";
import { buildMemoListFilter } from "@/utils/memoListFilter";

const PAGE_SIZE = 100;

const formatDateTimeLocal = (value: dayjs.Dayjs) => value.format("YYYY-MM-DDTHH:mm");

const parseDateTimeLocal = (value: string) => {
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed : null;
};

const escapeMarkdownText = (value: string) => value.replace(/\|/g, "\\|").trim();

const formatEmotionLabel = (emotion: Emotion, t: (key: any) => string) => {
  const option = EMOTION_OPTIONS.find((item) => item.value === emotion);
  return option ? `${option.emoji} ${t(option.labelKey as any)}` : emotion;
};

const buildExportMarkdown = ({
  exportedMemos,
  criteria,
  exportedAt,
  t,
}: {
  exportedMemos: Memo[];
  criteria: {
    startTime: string;
    endTime: string;
    contentSearch: string;
    tags: string[];
    emotions: Emotion[];
    anyEmotion: boolean;
    hasLink: boolean;
    hasTaskList: boolean;
    hasCode: boolean;
    pinnedOnly: boolean;
  };
  exportedAt: string;
  t: (key: any) => string;
}) => {
  const lines: string[] = [];
  lines.push(`# ${t("setting.export-section.title")}`);
  lines.push("");
  lines.push(`- ${t("setting.export-section.exported-at")}: ${exportedAt}`);
  lines.push(`- ${t("setting.export-section.start-time")}: ${criteria.startTime}`);
  lines.push(`- ${t("setting.export-section.end-time")}: ${criteria.endTime}`);
  if (criteria.contentSearch) {
    lines.push(`- ${t("setting.export-section.content-search")}: ${escapeMarkdownText(criteria.contentSearch)}`);
  }
  if (criteria.tags.length > 0) {
    lines.push(`- ${t("setting.export-section.tags")}: ${criteria.tags.map(escapeMarkdownText).join(", ")}`);
  }
  lines.push(
    `- ${t("setting.export-section.emotion-filter")}: ${
      criteria.anyEmotion
        ? t("setting.export-section.any-emotion")
        : criteria.emotions.length > 0
          ? criteria.emotions.map((emotion) => formatEmotionLabel(emotion, t)).join(", ")
          : t("setting.export-section.all-emotions")
    }`,
  );
  const flags = [
    criteria.pinnedOnly ? t("setting.export-section.pinned-filter") : "",
    criteria.hasLink ? t("setting.export-section.link-filter") : "",
    criteria.hasTaskList ? t("setting.export-section.task-filter") : "",
    criteria.hasCode ? t("setting.export-section.code-filter") : "",
  ].filter(Boolean);
  if (flags.length > 0) {
    lines.push(`- ${t("setting.export-section.filters")}: ${flags.join(", ")}`);
  }
  lines.push("");
  lines.push(`## ${t("setting.export-section.memo-list")} (${exportedMemos.length})`);
  lines.push("");

  for (const memo of exportedMemos) {
    const memoSourceTime = memo.displayTime || memo.updateTime || memo.createTime;
    const memoTime = memoSourceTime ? dayjs(memoSourceTime).format("YYYY-MM-DD HH:mm") : t("common.null");
    lines.push(`### ${memoTime}`);
    lines.push("");
    lines.push(`- ${t("setting.export-section.memo-emotion")}: ${formatEmotionLabel(memo.emotion, t)}`);
    if (memo.tags.length > 0) {
      lines.push(`- ${t("setting.export-section.memo-tags")}: ${memo.tags.map(escapeMarkdownText).join(", ")}`);
    }
    if (memo.pinned) {
      lines.push(`- ${t("common.pinned")}`);
    }
    lines.push("");
    lines.push(memo.content?.trim() || memo.snippet || t("setting.export-section.empty-content"));
    lines.push("");
    lines.push("---");
    lines.push("");
  }

  return lines.join("\n");
};

const ExportSection = () => {
  const t = useTranslate();
  const currentUser = useCurrentUser();
  const [startTime, setStartTime] = useState(() => formatDateTimeLocal(dayjs().subtract(30, "day").startOf("day")));
  const [endTime, setEndTime] = useState(() => formatDateTimeLocal(dayjs().endOf("day")));
  const [contentSearch, setContentSearch] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [selectedEmotions, setSelectedEmotions] = useState<Emotion[]>([]);
  const [anyEmotionOnly, setAnyEmotionOnly] = useState(false);
  const [hasLink, setHasLink] = useState(false);
  const [hasTaskList, setHasTaskList] = useState(false);
  const [hasCode, setHasCode] = useState(false);
  const [pinnedOnly, setPinnedOnly] = useState(false);
  const [isExporting, setIsExporting] = useState(false);

  const tags = useMemo(
    () =>
      tagInput
        .split(/[,，\n]/)
        .map((tag) => tag.trim())
        .filter(Boolean),
    [tagInput],
  );

  const filterExpression = useMemo(() => {
    const filters: MemoFilter[] = [];

    if (contentSearch.trim()) {
      filters.push({ factor: "contentSearch", value: contentSearch.trim() });
    }
    for (const tag of tags) {
      filters.push({ factor: "tagSearch", value: tag });
    }
    for (const emotion of anyEmotionOnly ? [ANY_EMOTION_FILTER] : selectedEmotions) {
      filters.push({ factor: "emotionSearch", value: emotion });
    }
    if (pinnedOnly) {
      filters.push({ factor: "pinned", value: "" });
    }
    if (hasLink) {
      filters.push({ factor: "property.hasLink", value: "" });
    }
    if (hasTaskList) {
      filters.push({ factor: "property.hasTaskList", value: "" });
    }
    if (hasCode) {
      filters.push({ factor: "property.hasCode", value: "" });
    }

    const baseFilter = buildMemoListFilter({ filters });
    const start = parseDateTimeLocal(startTime);
    const end = parseDateTimeLocal(endTime);
    const conditions: string[] = [];

    if (baseFilter) {
      conditions.push(baseFilter);
    }
    if (start) {
      conditions.push(`display_time_after == ${Math.floor(start.valueOf() / 1000)}`);
    }
    if (end) {
      conditions.push(`display_time_before == ${Math.floor(end.valueOf() / 1000)}`);
    }

    return conditions.join(" && ");
  }, [anyEmotionOnly, contentSearch, endTime, hasCode, hasTaskList, hasLink, pinnedOnly, selectedEmotions, startTime, tags]);

  const selectedEmotionLabels = useMemo(() => {
    return selectedEmotions.map((emotion) => t(EMOTION_OPTIONS.find((option) => option.value === emotion)?.labelKey as any));
  }, [selectedEmotions, t]);

  const toggleEmotion = (emotion: Emotion) => {
    setAnyEmotionOnly(false);
    setSelectedEmotions((current) => (current.includes(emotion) ? current.filter((item) => item !== emotion) : [...current, emotion]));
  };

  const toggleAnyEmotion = () => {
    setAnyEmotionOnly((current) => {
      const next = !current;
      if (next) {
        setSelectedEmotions([]);
      }
      return next;
    });
  };

  const handleExport = async () => {
    if (!currentUser) {
      return;
    }

    const start = parseDateTimeLocal(startTime);
    const end = parseDateTimeLocal(endTime);
    if (!start || !end || start.valueOf() > end.valueOf()) {
      toast.error(t("setting.export-section.invalid-range"));
      return;
    }

    setIsExporting(true);
    try {
      const exportedMemos: Memo[] = [];
      exportedMemos.push(
        ...(await listAllMemos({
          parent: currentUser.name,
          pageSize: PAGE_SIZE,
          state: State.NORMAL,
          direction: Direction.DESC,
          filter: "",
          oldFilter: filterExpression,
        })),
      );

      const criteria = {
        startTime: start.toISOString(),
        endTime: end.toISOString(),
        contentSearch: contentSearch.trim(),
        tags,
        emotions: getEmotionSearchValues(anyEmotionOnly ? [ANY_EMOTION_FILTER] : selectedEmotions),
        anyEmotion: anyEmotionOnly,
        hasLink,
        hasTaskList,
        hasCode,
        pinnedOnly,
      };

      const markdown = buildExportMarkdown({
        exportedMemos,
        criteria,
        exportedAt: new Date().toISOString(),
        t,
      });

      const blob = new Blob([markdown], {
        type: "text/markdown;charset=utf-8",
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `memos-export-${dayjs().format("YYYYMMDD-HHmmss")}.md`;
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      toast.success(t("setting.export-section.export-success", { count: exportedMemos.length }));
    } catch (error: any) {
      toast.error(getErrorMessage(error, t("setting.export-section.export-failed")));
    } finally {
      setIsExporting(false);
    }
  };

  if (!currentUser) {
    return null;
  }

  return (
    <div className="w-full flex flex-col gap-4 pt-2 pb-4">
      <div className="space-y-1">
        <p className="font-medium text-gray-700 dark:text-gray-400">{t("setting.export-section.title")}</p>
        <p className="text-sm text-gray-700 dark:text-gray-500">{t("setting.export-section.description")}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label className="text-sm text-gray-700 dark:text-gray-400">
          <span>{t("setting.export-section.start-time")}</span>
          <input
            type="datetime-local"
            className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
            value={startTime}
            onChange={(event) => setStartTime(event.target.value)}
          />
        </label>

        <label className="text-sm text-gray-700 dark:text-gray-400">
          <span>{t("setting.export-section.end-time")}</span>
          <input
            type="datetime-local"
            className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
            value={endTime}
            onChange={(event) => setEndTime(event.target.value)}
          />
        </label>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label className="text-sm text-gray-700 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <SearchIcon className="h-4 w-4 opacity-60" />
            {t("setting.export-section.content-search")}
          </span>
          <input
            className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
            placeholder={t("setting.export-section.content-search-placeholder")}
            value={contentSearch}
            onChange={(event) => setContentSearch(event.target.value)}
          />
        </label>

        <label className="text-sm text-gray-700 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <TagIcon className="h-4 w-4 opacity-60" />
            {t("setting.export-section.tags")}
          </span>
          <input
            className="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-200 dark:focus:border-blue-400 dark:focus:ring-zinc-700"
            placeholder={t("setting.export-section.tags-placeholder")}
            value={tagInput}
            onChange={(event) => setTagInput(event.target.value)}
          />
        </label>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-1 text-sm text-gray-700 dark:text-gray-400">
          <SmilePlusIcon className="h-4 w-4 opacity-60" />
          <span>{t("setting.export-section.emotion")}</span>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            className={cn(
              "inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-sm transition",
              anyEmotionOnly
                ? "border-blue-300 bg-blue-50 text-blue-700 shadow-sm dark:border-blue-700 dark:bg-blue-950/50 dark:text-blue-200"
                : "border-gray-200 bg-white text-gray-600 hover:border-blue-200 hover:text-blue-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-400",
            )}
            onClick={toggleAnyEmotion}
          >
            <span>{t("setting.export-section.any-emotion")}</span>
          </button>
          {EMOTION_OPTIONS.map((option) => {
            const active = !anyEmotionOnly && selectedEmotions.includes(option.value);
            return (
              <button
                key={option.value}
                type="button"
                className={cn(
                  "inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-sm transition",
                  active
                    ? "border-blue-300 bg-blue-50 text-blue-700 shadow-sm dark:border-blue-700 dark:bg-blue-950/50 dark:text-blue-200"
                    : "border-gray-200 bg-white text-gray-600 hover:border-blue-200 hover:text-blue-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-gray-400",
                )}
                onClick={() => toggleEmotion(option.value)}
              >
                <span>{option.emoji}</span>
                <span>{t(option.labelKey as any)}</span>
              </button>
            );
          })}
        </div>
        {(anyEmotionOnly || selectedEmotionLabels.length > 0) && (
          <div className="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-500">
            <span>{t("setting.export-section.emotion-filter")}:</span>
            {anyEmotionOnly ? (
              <span className="rounded-full bg-gray-100 px-2 py-0.5 dark:bg-zinc-800">{t("setting.export-section.any-emotion")}</span>
            ) : (
              selectedEmotionLabels.map((label) => (
                <span key={label} className="rounded-full bg-gray-100 px-2 py-0.5 dark:bg-zinc-800">
                  {label}
                </span>
              ))
            )}
          </div>
        )}
        {selectedEmotionLabels.length === 0 && (
          <div className="text-xs text-gray-500 dark:text-gray-500">
            {t("setting.export-section.emotion-filter")}: {t("setting.export-section.all-emotions")}
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
          <Switch checked={hasLink} onChange={(event) => setHasLink(event.target.checked)} />
          <span className="flex items-center gap-1">
            <LinkIcon className="h-4 w-4 opacity-60" />
            {t("setting.export-section.link-filter")}
          </span>
        </label>
        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
          <Switch checked={hasTaskList} onChange={(event) => setHasTaskList(event.target.checked)} />
          <span>{t("setting.export-section.task-filter")}</span>
        </label>
        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
          <Switch checked={hasCode} onChange={(event) => setHasCode(event.target.checked)} />
          <span>{t("setting.export-section.code-filter")}</span>
        </label>
        <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-400">
          <Switch checked={pinnedOnly} onChange={(event) => setPinnedOnly(event.target.checked)} />
          <span>{t("setting.export-section.pinned-filter")}</span>
        </label>
      </div>

      <div className="flex flex-wrap items-center gap-2 pt-1">
        <Button color="primary" disabled={isExporting} onClick={handleExport}>
          <DownloadIcon className="mr-1 h-4 w-4" />
          {isExporting ? t("setting.export-section.exporting") : t("setting.export-section.export-markdown")}
        </Button>
      </div>
    </div>
  );
};

export default ExportSection;
