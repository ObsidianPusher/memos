import { ChevronDownIcon, SmilePlusIcon } from "lucide-react";
import { useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/Popover";
import { Emotion } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";
import { EMOTION_META, EMOTION_OPTIONS, isEmotionMemo } from "@/utils/emotion";
import { useTranslate } from "@/utils/i18n";

interface Props {
  value: Emotion;
  onChange: (emotion: Emotion) => void;
  onOpenChange?: (open: boolean) => void;
  className?: string;
}

const EmotionSelector = (props: Props) => {
  const { value, onChange } = props;
  const t = useTranslate();
  const [open, setOpen] = useState(false);
  const currentMeta = EMOTION_META[value];
  const hasEmotion = isEmotionMemo(value);

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    props.onOpenChange?.(nextOpen);
  };

  const handleSelect = (emotion: Emotion) => {
    onChange(emotion);
    handleOpenChange(false);
  };

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <button
          type="button"
          title={t("common.emotions")}
          className={cn(
            "inline-flex h-8 items-center gap-1.5 rounded-lg px-3 text-sm font-medium transition-colors",
            "border border-gray-200 bg-white shadow-sm hover:border-gray-300 hover:bg-gray-50",
            hasEmotion
              ? "border-gray-300 text-gray-900 dark:border-zinc-600 dark:bg-zinc-700/80 dark:text-zinc-100"
              : "text-gray-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:border-zinc-600 dark:hover:bg-zinc-700/70",
            props.className,
          )}
        >
          <SmilePlusIcon className="h-4 w-4 shrink-0" />
          <span>{hasEmotion ? t(currentMeta.labelKey as any) : t("common.emotions")}</span>
          <ChevronDownIcon className="h-3.5 w-3.5 opacity-60" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="!p-1.5" align="end" sideOffset={4}>
        <div className="mb-1 flex items-center justify-between px-1 pt-0.5">
          <p className="text-xs font-medium text-gray-500 dark:text-gray-400">{t("common.select")}</p>
          {value !== Emotion.EMOTION_UNSPECIFIED && (
            <button
              type="button"
              onClick={() => handleSelect(Emotion.EMOTION_UNSPECIFIED)}
              className="text-xs font-medium text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-zinc-100"
            >
              {t("common.clear")}
            </button>
          )}
        </div>
        <div className="grid min-w-40 grid-cols-1 gap-0.5">
          {EMOTION_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => handleSelect(option.value)}
              className={cn(
                "flex items-center gap-2.5 rounded-xl px-2.5 py-2 text-left text-sm transition-colors",
                "hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500",
                "dark:hover:bg-zinc-700/70",
                option.value === value ? "bg-gray-100 dark:bg-zinc-700/80" : "",
              )}
            >
              <span className={cn("inline-flex h-8 w-8 items-center justify-center rounded-full text-sm", option.color)}>
                {option.emoji}
              </span>
              <div className="flex flex-col">
                <span className="font-medium text-gray-900 dark:text-gray-100">{t(option.labelKey as any)}</span>
              </div>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
};

export default EmotionSelector;
