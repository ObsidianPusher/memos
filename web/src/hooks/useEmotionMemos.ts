import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { fetchEmotionReport, type EmotionReportResponse } from "@/api/memo";
import useCurrentUser from "@/hooks/useCurrentUser";
import { memoFilterStore, userStore } from "@/store/v2";
import { getErrorMessage } from "@/utils/error";
import { buildMemoListFilter } from "@/utils/memoListFilter";

export type EmotionReportGranularity = "week" | "month";

export const useEmotionMemos = (reportRange: number, reportGranularity: EmotionReportGranularity) => {
  const currentUser = useCurrentUser();
  const [report, setReport] = useState<EmotionReportResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const selectedShortcut = userStore.state.shortcuts.find((shortcut) => shortcut.id === memoFilterStore.shortcut);

  const memoListFilter = useMemo(
    () => buildMemoListFilter({ filters: memoFilterStore.filters, fallbackEmotionSearch: true }),
    [memoFilterStore.filters],
  );
  const reportWindow = useMemo(
    () => ({
      startTime: dayjs()
        .subtract(reportRange - 1, "day")
        .startOf("day")
        .format("YYYY-MM-DDTHH:mm:ssZ"),
      endTime: dayjs().endOf("day").format("YYYY-MM-DDTHH:mm:ssZ"),
    }),
    [reportRange],
  );

  useEffect(() => {
    if (!currentUser) {
      setReport(null);
      setIsLoading(false);
      setError("");
      return;
    }

    const controller = new AbortController();
    setIsLoading(true);
    setError("");
    setReport(null);

    void fetchEmotionReport({
      startTime: reportWindow.startTime,
      endTime: reportWindow.endTime,
      granularity: reportGranularity,
      filter: selectedShortcut?.filter || "",
      oldFilter: memoListFilter,
      signal: controller.signal,
    })
      .then((nextReport) => {
        if (!controller.signal.aborted) {
          setReport(nextReport);
        }
      })
      .catch((err: unknown) => {
        if (!controller.signal.aborted) {
          setError(getErrorMessage(err));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setIsLoading(false);
        }
      });

    return () => {
      controller.abort();
    };
  }, [currentUser?.name, memoListFilter, reportGranularity, reportWindow.endTime, reportWindow.startTime, selectedShortcut?.filter]);

  return { report, isLoading, error, currentUser };
};
