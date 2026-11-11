import { observer } from "mobx-react-lite";
import { useState } from "react";
import type { EmotionReportResponse } from "@/api/memo";
import EmotionChart from "@/components/EmotionChart";
import MobileHeader from "@/components/MobileHeader";
import { useEmotionMemos } from "@/hooks/useEmotionMemos";
import { EmotionLegendRow, SummaryChip, StateCard } from "@/pages/emotion/components";
import { Emotion } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";
import { EMOTION_META } from "@/utils/emotion";
import { useTranslate } from "@/utils/i18n";

type EmotionReportRange = 7 | 30 | 90;
type EmotionReportGranularity = "week" | "month";

const REPORT_RANGE_OPTIONS: EmotionReportRange[] = [7, 30, 90];
const REPORT_GRANULARITY_OPTIONS: EmotionReportGranularity[] = ["week", "month"];

const EmotionPage = observer(() => {
  const t = useTranslate();
  const [reportRange, setReportRange] = useState<EmotionReportRange>(30);
  const [reportGranularity, setReportGranularity] = useState<EmotionReportGranularity>("week");
  const { report, isLoading, error, currentUser } = useEmotionMemos(reportRange, reportGranularity);
  const reportSummary: EmotionReportResponse = report ?? {
    total: 0,
    averageScore: 0,
    dominantEmotion: Emotion.EMOTION_UNSPECIFIED,
    breakdown: [],
    series: [],
  };
  const dominantEmotionLabel =
    reportSummary.total === 0 ? t("message.no-data") : t(EMOTION_META[reportSummary.dominantEmotion].labelKey as any) || "";
  const averageScoreLabel = reportSummary.total === 0 ? "-" : reportSummary.averageScore.toFixed(2);
  const reportWindowLabel = `${reportRange}D · ${t(`memo.range-${reportGranularity}` as any)}`;

  if (!currentUser) {
    return null;
  }

  return (
    <section className="relative w-full min-h-full overflow-hidden pt-2 sm:pt-3">
      <div className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-72 bg-gradient-to-b from-sky-50/80 via-white to-transparent dark:from-sky-950/20 dark:via-zinc-950 dark:to-transparent" />
      <div className="xl:hidden">
        <MobileHeader />
      </div>

      <div className="mx-auto w-full max-w-[1480px] px-4 pb-8 sm:px-6 lg:px-8">
        <section className="overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="flex flex-col gap-4 border-b border-gray-200 px-5 py-5 dark:border-zinc-800 sm:px-6 xl:flex-row xl:items-end xl:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2 text-xs font-medium">
                <span className="inline-flex items-center rounded-full bg-sky-50 px-2.5 py-1 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">
                  {t("common.emotions")}
                </span>
                <span className="rounded-full bg-gray-100 px-2.5 py-1 text-gray-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {reportWindowLabel}
                </span>
                <span className="rounded-full bg-gray-100 px-2.5 py-1 text-gray-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {t("memo.chart")}
                </span>
              </div>
              <h1 className="mt-3 text-2xl font-semibold tracking-tight text-gray-900 dark:text-zinc-100 sm:text-3xl">{t("memo.chart")}</h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">{t("memo.emotion-page-description")}</p>
            </div>

            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:min-w-[36rem]">
              <SummaryChip label={t("common.memos")} value={String(reportSummary.total)} />
              <SummaryChip label={t("memo.average-score")} value={averageScoreLabel} />
              <SummaryChip label={t("memo.dominant-emotion")} value={dominantEmotionLabel} />
              <SummaryChip label={t("memo.chart")} value={reportWindowLabel} />
            </div>
          </div>

          <div className="flex flex-col gap-3 px-5 py-4 sm:px-6 xl:flex-row xl:items-center xl:justify-between">
            <div className="inline-flex w-full items-stretch gap-1 rounded-[1.25rem] bg-gray-100 p-1 dark:bg-zinc-800 xl:w-auto">
              {REPORT_RANGE_OPTIONS.map((days) => {
                const active = reportRange === days;
                return (
                  <button
                    key={days}
                    type="button"
                    onClick={() => setReportRange(days)}
                    className={cn(
                      "flex-1 rounded-2xl px-4 py-2 text-sm font-medium transition-colors xl:flex-none",
                      active
                        ? "bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-zinc-100"
                        : "text-gray-500 hover:text-gray-900 dark:text-zinc-400 dark:hover:text-zinc-100",
                    )}
                  >
                    {days}D
                  </button>
                );
              })}
            </div>

            <div className="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-gray-50 p-1 dark:border-zinc-700 dark:bg-zinc-800">
              {REPORT_GRANULARITY_OPTIONS.map((mode) => {
                const active = reportGranularity === mode;
                return (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => setReportGranularity(mode)}
                    className={cn(
                      "rounded-full px-3 py-1.5 text-xs font-medium transition-colors",
                      active
                        ? "bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-zinc-50"
                        : "text-gray-500 hover:text-gray-900 dark:text-zinc-400 dark:hover:text-zinc-100",
                    )}
                  >
                    {t(`memo.range-${mode}` as any)}
                  </button>
                );
              })}
            </div>
          </div>
        </section>

        <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.7fr)_minmax(340px,0.9fr)] xl:items-start">
          <main className="space-y-4">
            <section className="overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="border-b border-gray-200 px-5 py-4 dark:border-zinc-800 sm:px-6">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <p className="text-xs font-medium uppercase tracking-[0.24em] text-gray-500 dark:text-gray-400">{t("memo.chart")}</p>
                    <h2 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-zinc-100 sm:text-2xl">
                      {t("memo.chart")}
                    </h2>
                  </div>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    {reportSummary.total} {t("common.memos")} · {averageScoreLabel}
                  </p>
                </div>
              </div>

              <div className="px-4 py-4 sm:px-6 sm:py-6">
                {isLoading ? (
                  <StateCard title={t("memo.emotion-loading")} />
                ) : error ? (
                  <StateCard title={error} tone="danger" />
                ) : reportSummary.series.length > 0 ? (
                  <EmotionChart points={reportSummary.series} range={reportGranularity} />
                ) : (
                  <StateCard title={t("memo.emotion-empty")} />
                )}
              </div>
            </section>
          </main>

          <aside className="space-y-4">
            <section className="overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="border-b border-gray-200 px-5 py-4 dark:border-zinc-800">
                <p className="text-sm font-medium text-gray-900 dark:text-zinc-100">{t("memo.dominant-emotion")}</p>
              </div>
              <div className="space-y-3 p-4">
                {reportSummary.breakdown.map((item) => {
                  const meta = EMOTION_META[item.emotion];
                  const label = t(meta.labelKey as any) || "";
                  return (
                    <EmotionLegendRow
                      key={item.emotion}
                      label={label}
                      emoji={meta.emoji}
                      count={item.count}
                      ratio={item.ratio}
                      toneClass={getEmotionDotClass(item.emotion)}
                    />
                  );
                })}
              </div>
            </section>

            <section className="overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="border-b border-gray-200 px-5 py-4 dark:border-zinc-800">
                <p className="text-sm font-medium text-gray-900 dark:text-zinc-100">报表摘要</p>
              </div>
              <div className="grid gap-2 p-4 sm:grid-cols-2 xl:grid-cols-1">
                <SummaryChip label={t("common.memos")} value={String(reportSummary.total)} />
                <SummaryChip label={t("memo.average-score")} value={averageScoreLabel} />
                <SummaryChip label={t("memo.dominant-emotion")} value={dominantEmotionLabel} />
                <SummaryChip label="区间" value={reportWindowLabel} />
              </div>
            </section>
          </aside>
        </div>
      </div>
    </section>
  );
});

const getEmotionDotClass = (emotion: Emotion) => {
  switch (emotion) {
    case Emotion.TERRIBLE:
      return "bg-rose-500 text-rose-50";
    case Emotion.BAD:
      return "bg-amber-500 text-amber-50";
    case Emotion.OKAY:
      return "bg-sky-500 text-sky-50";
    case Emotion.GOOD:
      return "bg-emerald-500 text-emerald-50";
    case Emotion.AWESOME:
      return "bg-violet-500 text-violet-50";
    default:
      return "bg-gray-500 text-white";
  }
};

export default EmotionPage;
