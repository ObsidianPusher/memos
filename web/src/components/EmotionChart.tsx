import { useMemo, useState } from "react";
import { Emotion } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";
import { EMOTION_META } from "@/utils/emotion";
import { useTranslate } from "@/utils/i18n";

interface Point {
  labelLines: string[];
  value: number;
  count: number;
  dominantEmotion: Emotion;
}

interface Props {
  points: Point[];
  range: "week" | "month";
}

const EmotionChart = ({ points, range }: Props) => {
  const t = useTranslate();
  const [hoveredIndex, setHoveredIndex] = useState<number>(-1);

  const chart = useMemo(() => buildChart(points), [points]);
  const hoveredPoint = hoveredIndex >= 0 ? chart.points[hoveredIndex] : undefined;
  const rangeLabel = t(`memo.range-${range}` as any);

  return (
    <div className="overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <div className="border-b border-gray-200 px-5 py-4 dark:border-zinc-800">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-gray-900 dark:text-zinc-100">{t("memo.chart")}</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">{rangeLabel}</p>
          </div>
          <div className="text-right text-xs text-gray-500 dark:text-gray-400">
            {points.reduce((sum, point) => sum + point.count, 0)} {t("common.memos")}
          </div>
        </div>
      </div>

      <div className="relative overflow-x-auto px-3 py-4 sm:px-5" onMouseLeave={() => setHoveredIndex(-1)}>
        {hoveredPoint && (
          <div className="pointer-events-none absolute left-5 top-5 z-10 w-[min(18rem,calc(100%-2.5rem))] rounded-2xl border border-gray-200 bg-white/95 px-3 py-2 text-sm shadow-lg backdrop-blur dark:border-zinc-700 dark:bg-zinc-900/95">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">{hoveredPoint.labelLines.join(" · ")}</p>
            <div className="mt-1 flex items-end justify-between gap-2">
              <div>
                <p className="text-xl font-semibold text-gray-900 dark:text-zinc-100">{hoveredPoint.value.toFixed(2)}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  {hoveredPoint.count} {t("common.memos")}
                </p>
              </div>
              <div className="flex items-center gap-1 rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-zinc-800 dark:text-zinc-200">
                <span>{EMOTION_META[hoveredPoint.dominantEmotion].emoji}</span>
                <span>{t(EMOTION_META[hoveredPoint.dominantEmotion].labelKey as any) || ""}</span>
              </div>
            </div>
          </div>
        )}

        <svg viewBox={`0 0 ${chart.width} ${chart.height}`} className="h-[280px] sm:h-[320px]" style={{ width: `${chart.width}px` }}>
          <defs>
            <linearGradient id="emotion-line" x1="0" x2="1" y1="0" y2="0">
              <stop offset="0%" stopColor="rgb(59 130 246)" />
              <stop offset="100%" stopColor="rgb(16 185 129)" />
            </linearGradient>
            <linearGradient id="emotion-fill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor="rgb(59 130 246)" stopOpacity="0.26" />
              <stop offset="100%" stopColor="rgb(16 185 129)" stopOpacity="0.02" />
            </linearGradient>
          </defs>

          {chart.gridLines.map((line) => (
            <g key={line.value}>
              <line
                x1={chart.paddingLeft}
                x2={chart.width - chart.paddingRight}
                y1={line.y}
                y2={line.y}
                className={cn(line.value === 3 ? "stroke-gray-300 dark:stroke-zinc-700" : "stroke-gray-200 dark:stroke-zinc-800")}
              />
              <text x={chart.paddingLeft - 10} y={line.y + 4} textAnchor="end" className="fill-gray-400 text-[10px] dark:fill-zinc-500">
                {line.value}
              </text>
            </g>
          ))}

          <path d={chart.areaPath} fill="url(#emotion-fill)" />
          <path d={chart.linePath} fill="none" stroke="url(#emotion-line)" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" />
          {chart.points.map((point) => (
            <g key={point.index} onMouseEnter={() => setHoveredIndex(point.index)}>
              <circle
                cx={point.x}
                cy={point.y}
                r={hoveredIndex === point.index ? "7" : "5.5"}
                fill={hoveredIndex === point.index ? "rgb(255 255 255)" : "white"}
                stroke="url(#emotion-line)"
                strokeWidth={hoveredIndex === point.index ? "4" : "3"}
              />
              <line
                x1={point.x}
                x2={point.x}
                y1={point.y + 8}
                y2={chart.height - chart.paddingBottom + 4}
                className={cn(point.showLabel ? "stroke-gray-300 dark:stroke-zinc-700" : "stroke-gray-200/50 dark:stroke-zinc-800/40")}
              />
              <title>
                {point.labelLines.join(" / ")} - {point.value.toFixed(2)}
              </title>
            </g>
          ))}

          {chart.labels.map((label) =>
            label.show ? (
              <text
                key={label.index}
                x={label.x}
                y={chart.height - 14}
                textAnchor="middle"
                className="fill-gray-500 text-[10px] dark:fill-zinc-400"
              >
                {label.lines.map((line, index) => (
                  <tspan key={`${label.index}-${index}`} x={label.x} dy={index === 0 ? 0 : 11}>
                    {line}
                  </tspan>
                ))}
              </text>
            ) : null,
          )}
        </svg>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-gray-200 px-5 py-4 text-xs text-gray-500 dark:border-zinc-800 dark:text-zinc-400">
        <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 dark:bg-zinc-800">{rangeLabel}</span>
        <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 dark:bg-zinc-800">
          {t("memo.average-score")} · 1-5
        </span>
      </div>
    </div>
  );
};

const buildChart = (points: Point[]) => {
  const width = Math.max(680, points.length * 96);
  const height = 320;
  const paddingLeft = 42;
  const paddingRight = 24;
  const paddingTop = 24;
  const paddingBottom = 48;
  const chartWidth = width - paddingLeft - paddingRight;
  const chartHeight = height - paddingTop - paddingBottom;
  const min = 1;
  const max = 5;
  const count = points.length;
  const step = count > 1 ? chartWidth / (count - 1) : 0;

  const visibleLabelBudget = Math.min(8, Math.max(4, Math.floor(chartWidth / 110)));
  const labelStep = count <= visibleLabelBudget ? 1 : Math.ceil(count / visibleLabelBudget);

  const mapped = points.map((point, index) => {
    const x = paddingLeft + step * index;
    const normalized = Math.min(Math.max(point.value, min), max);
    const y = paddingTop + chartHeight - ((normalized - min) / (max - min)) * chartHeight;
    return {
      ...point,
      index,
      x,
      y,
      showLabel: index === 0 || index === count - 1 || index % labelStep === 0,
    };
  });

  const linePath = mapped.length
    ? `M ${mapped[0].x} ${mapped[0].y} ${mapped
        .slice(1)
        .map((point) => `L ${point.x} ${point.y}`)
        .join(" ")}`
    : "";

  const areaPath = mapped.length
    ? `${linePath} L ${mapped[mapped.length - 1].x} ${height - paddingBottom} L ${mapped[0].x} ${height - paddingBottom} Z`
    : "";

  const gridLines = [1, 2, 3, 4, 5].map((value) => {
    const y = paddingTop + chartHeight - ((value - min) / (max - min)) * chartHeight;
    return { value, y };
  });

  const labels = mapped.map((point) => ({
    index: point.index,
    x: point.x,
    lines: point.labelLines,
    show: point.showLabel,
  }));

  return {
    width,
    height,
    paddingLeft,
    paddingRight,
    paddingBottom,
    gridLines,
    points: mapped,
    labels,
    linePath,
    areaPath,
  };
};

export default EmotionChart;
