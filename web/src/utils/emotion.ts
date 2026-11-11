import { Emotion } from "@/types/proto/api/v1/memo_service";

export const EMOTION_ORDER = [Emotion.TERRIBLE, Emotion.BAD, Emotion.OKAY, Emotion.GOOD, Emotion.AWESOME] as const;
export const ANY_EMOTION_FILTER = "__ANY_EMOTION__";

export const EMOTION_SCORE_MAP: Record<Emotion, number> = {
  [Emotion.EMOTION_UNSPECIFIED]: 0,
  [Emotion.TERRIBLE]: 1,
  [Emotion.BAD]: 2,
  [Emotion.OKAY]: 3,
  [Emotion.GOOD]: 4,
  [Emotion.AWESOME]: 5,
  [Emotion.UNRECOGNIZED]: 0,
};

export const EMOTION_META: Record<
  Emotion,
  {
    emoji: string;
    labelKey: string;
    color: string;
    score: number;
  }
> = {
  [Emotion.EMOTION_UNSPECIFIED]: {
    emoji: "•",
    labelKey: "memo.emotion.unspecified",
    color: "text-gray-400 bg-gray-100 dark:bg-zinc-700",
    score: 0,
  },
  [Emotion.TERRIBLE]: {
    emoji: "😭",
    labelKey: "memo.emotion.terrible",
    color: "text-rose-600 bg-rose-100 dark:bg-rose-950/70",
    score: 1,
  },
  [Emotion.BAD]: {
    emoji: "🙁",
    labelKey: "memo.emotion.bad",
    color: "text-amber-600 bg-amber-100 dark:bg-amber-950/70",
    score: 2,
  },
  [Emotion.OKAY]: {
    emoji: "😐",
    labelKey: "memo.emotion.okay",
    color: "text-sky-600 bg-sky-100 dark:bg-sky-950/70",
    score: 3,
  },
  [Emotion.GOOD]: {
    emoji: "🙂",
    labelKey: "memo.emotion.good",
    color: "text-emerald-600 bg-emerald-100 dark:bg-emerald-950/70",
    score: 4,
  },
  [Emotion.AWESOME]: {
    emoji: "🤩",
    labelKey: "memo.emotion.awesome",
    color: "text-violet-600 bg-violet-100 dark:bg-violet-950/70",
    score: 5,
  },
  [Emotion.UNRECOGNIZED]: {
    emoji: "•",
    labelKey: "memo.emotion.unspecified",
    color: "text-gray-400 bg-gray-100 dark:bg-zinc-700",
    score: 0,
  },
};

export const EMOTION_OPTIONS = EMOTION_ORDER.map((emotion) => ({
  value: emotion,
  ...EMOTION_META[emotion],
}));

export const getEmotionSearchValues = (values: string[]): Emotion[] => {
  if (values.includes(ANY_EMOTION_FILTER)) {
    return [...EMOTION_ORDER];
  }

  const emotionSet = new Set<string>(EMOTION_ORDER);
  return values.filter((value): value is Emotion => emotionSet.has(value));
};

export const getEmotionScore = (emotion: Emotion) => EMOTION_META[emotion]?.score ?? 0;

export const isEmotionMemo = (emotion: Emotion) => emotion !== Emotion.EMOTION_UNSPECIFIED && emotion !== Emotion.UNRECOGNIZED;
