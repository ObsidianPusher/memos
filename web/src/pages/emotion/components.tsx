import { cn } from "@/utils";

export const SummaryChip = ({ label, value }: { label: string; value: string }) => {
  return (
    <div className="min-w-0 rounded-2xl border border-gray-200 bg-gray-50/70 px-3 py-2 shadow-sm dark:border-zinc-800 dark:bg-zinc-900/70">
      <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-gray-500 dark:text-gray-400">{label}</p>
      <p className="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-zinc-100">{value}</p>
    </div>
  );
};

export const EmotionLegendRow = ({
  label,
  emoji,
  count,
  ratio,
  toneClass,
}: {
  label: string;
  emoji: string;
  count: number;
  ratio: number;
  toneClass: string;
}) => {
  return (
    <div className="space-y-2 rounded-2xl border border-gray-200 bg-gray-50/70 p-3 dark:border-zinc-800 dark:bg-zinc-900/70">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className={cn(
              "inline-flex h-8 w-8 items-center justify-center rounded-2xl border-4 border-white text-sm shadow-sm dark:border-zinc-900",
              toneClass,
            )}
          >
            {emoji}
          </span>
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-gray-900 dark:text-zinc-100">{label}</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">{ratio.toFixed(0)}%</p>
          </div>
        </div>
        <span className="inline-flex shrink-0 items-center rounded-full bg-gray-900 px-2.5 py-1 text-xs font-semibold text-white dark:bg-zinc-100 dark:text-zinc-900">
          {count}
        </span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-zinc-800">
        <div className={cn("h-full rounded-full", toneClass)} style={{ width: `${Math.max(ratio, count > 0 ? 8 : 0)}%` }} />
      </div>
    </div>
  );
};

export const StateCard = ({
  title,
  description,
  tone = "neutral",
}: {
  title: string;
  description?: string;
  tone?: "neutral" | "danger";
}) => {
  return (
    <div
      className={cn(
        "rounded-[1.75rem] border px-5 py-8 text-center",
        tone === "danger"
          ? "border-red-200 bg-red-50/70 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200"
          : "border-dashed border-gray-300 bg-gray-50/60 text-gray-600 dark:border-zinc-700 dark:bg-zinc-900/70 dark:text-zinc-400",
      )}
    >
      <p className="text-sm font-medium">{title}</p>
      {description ? <p className="mx-auto mt-2 max-w-lg text-sm leading-6">{description}</p> : null}
    </div>
  );
};
