import { Dropdown, Menu, MenuButton, MenuItem } from "@mui/joy";
import dayjs from "dayjs";
import { BookmarkIcon, CheckCircleIcon, ChevronDownIcon, Code2Icon, LinkIcon, ListTodoIcon, SmilePlusIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState, useCallback } from "react";
import { matchPath, useLocation } from "react-router-dom";
import useCurrentUser from "@/hooks/useCurrentUser";
import { useStatisticsData } from "@/hooks/useStatisticsData";
import { Routes } from "@/router/routes";
import { memoStore, userStore } from "@/store/v2";
import memoFilterStore, { FilterFactor } from "@/store/v2/memoFilter";
import { ANY_EMOTION_FILTER, EMOTION_META, EMOTION_OPTIONS, isEmotionMemo } from "@/utils/emotion";
import { useTranslate } from "@/utils/i18n";
import ActivityCalendar from "../ActivityCalendar";
import { MonthNavigator } from "./MonthNavigator";
import { StatCard } from "./StatCard";

const StatisticsView = observer(() => {
  const t = useTranslate();
  const location = useLocation();
  const currentUser = useCurrentUser();
  const { memoTypeStats, activityStats } = useStatisticsData();
  const [selectedDate] = useState(new Date());
  const [visibleMonthString, setVisibleMonthString] = useState(dayjs().format("YYYY-MM"));
  const emotionFilters = memoFilterStore.getFiltersByFactor("emotionSearch");
  const currentEmotionFilter = emotionFilters[0]?.value;
  const emotionLabel =
    currentEmotionFilter === ANY_EMOTION_FILTER
      ? t("memo.any-emotion")
      : currentEmotionFilter
        ? t(EMOTION_META[currentEmotionFilter as keyof typeof EMOTION_META]?.labelKey as any)
        : t("common.emotions");
  const emotionCount = memoStore.state.memos.filter((memo) => isEmotionMemo(memo.emotion)).length;

  const handleCalendarClick = useCallback((date: string) => {
    memoFilterStore.removeFilter((f) => f.factor === "displayTime");
    memoFilterStore.addFilter({ factor: "displayTime", value: date });
  }, []);

  const handleFilterClick = useCallback((factor: FilterFactor, value: string = "") => {
    memoFilterStore.addFilter({ factor, value });
  }, []);

  const handleEmotionFilterClick = useCallback((value: string) => {
    memoFilterStore.removeFilter((filter) => filter.factor === "emotionSearch");
    memoFilterStore.addFilter({ factor: "emotionSearch", value });
  }, []);

  const isRootPath = matchPath(Routes.ROOT, location.pathname);
  const hasPinnedMemos = currentUser && (userStore.state.currentUserStats?.pinnedMemos || []).length > 0;

  return (
    <div className="group w-full mt-2 space-y-1 text-zinc-600 dark:text-gray-400 animate-fade-in">
      <MonthNavigator visibleMonth={visibleMonthString} onMonthChange={setVisibleMonthString} />

      <div className="w-full animate-scale-in">
        <ActivityCalendar
          month={visibleMonthString}
          selectedDate={selectedDate.toDateString()}
          data={activityStats}
          onClick={handleCalendarClick}
        />
      </div>

      <div className="pt-1 w-full flex flex-row justify-start items-center gap-1 flex-wrap">
        {isRootPath && hasPinnedMemos && (
          <StatCard
            icon={<BookmarkIcon className="w-4 h-auto mr-1 opacity-70" />}
            label={t("common.pinned")}
            count={userStore.state.currentUserStats!.pinnedMemos.length}
            onClick={() => handleFilterClick("pinned")}
          />
        )}

        <StatCard
          icon={<LinkIcon className="w-4 h-auto mr-1 opacity-70" />}
          label={t("memo.links")}
          count={memoTypeStats.linkCount}
          onClick={() => handleFilterClick("property.hasLink")}
        />

        <Dropdown>
          <MenuButton slots={{ root: "div" }}>
            <div className="w-auto border dark:border-zinc-800 pl-1.5 pr-2 py-0.5 rounded-md flex justify-between items-center cursor-pointer hover:bg-gray-50 dark:hover:bg-zinc-800/50 transition-colors">
              <div className="w-auto flex justify-start items-center mr-1">
                <SmilePlusIcon className="w-4 h-auto mr-1 opacity-70" />
                <span className="block text-sm opacity-80">{emotionLabel}</span>
              </div>
              <span className="text-sm truncate opacity-80 flex items-center gap-1">
                {emotionCount}
                <ChevronDownIcon className="w-3.5 h-auto opacity-40" />
              </span>
            </div>
          </MenuButton>
          <Menu size="sm" placement="bottom-start">
            <MenuItem onClick={() => handleEmotionFilterClick(ANY_EMOTION_FILTER)}>{t("memo.any-emotion")}</MenuItem>
            {EMOTION_OPTIONS.map((option) => (
              <MenuItem key={option.value} onClick={() => handleEmotionFilterClick(option.value)}>
                <span className="mr-1">{option.emoji}</span>
                {t(option.labelKey as any)}
              </MenuItem>
            ))}
            {emotionFilters.length > 0 && (
              <MenuItem color="neutral" onClick={() => memoFilterStore.removeFilter((filter) => filter.factor === "emotionSearch")}>
                {t("common.clear")}
              </MenuItem>
            )}
          </Menu>
        </Dropdown>

        <StatCard
          icon={
            memoTypeStats.undoCount > 0 ? (
              <ListTodoIcon className="w-4 h-auto mr-1 opacity-70" />
            ) : (
              <CheckCircleIcon className="w-4 h-auto mr-1 opacity-70" />
            )
          }
          label={t("memo.to-do")}
          count={
            memoTypeStats.undoCount > 0 ? (
              <div className="text-sm flex flex-row items-start justify-center">
                <span className="truncate">{memoTypeStats.todoCount - memoTypeStats.undoCount}</span>
                <span className="font-mono opacity-50">/</span>
                <span className="truncate">{memoTypeStats.todoCount}</span>
              </div>
            ) : (
              memoTypeStats.todoCount
            )
          }
          onClick={() => handleFilterClick("property.hasTaskList")}
          tooltip={memoTypeStats.undoCount > 0 ? "Done / Total" : undefined}
        />

        <StatCard
          icon={<Code2Icon className="w-4 h-auto mr-1 opacity-70" />}
          label={t("memo.code")}
          count={memoTypeStats.codeCount}
          onClick={() => handleFilterClick("property.hasCode")}
        />
      </div>
    </div>
  );
});

export default StatisticsView;
