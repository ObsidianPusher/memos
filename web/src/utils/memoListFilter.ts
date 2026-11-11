import dayjs from "dayjs";
import { type FilterFactor, type MemoFilter } from "@/store/v2/memoFilter";
import { EMOTION_ORDER, getEmotionSearchValues } from "@/utils/emotion";

const FILTER_VALUE_QUOTE = (value: string) => JSON.stringify(value);
const SECONDS_PER_DAY = 24 * 60 * 60;

interface BuildMemoListFilterOptions {
  filters: MemoFilter[];
  fallbackEmotionSearch?: boolean;
}

const buildDisplayTimeCondition = (value: string) => {
  const filterDate = new Date(value);
  if (Number.isNaN(filterDate.getTime())) {
    return [];
  }

  const filterUtcTimestamp = filterDate.getTime() + filterDate.getTimezoneOffset() * 60 * 1000;
  const timestampAfter = filterUtcTimestamp / 1000;

  return [`display_time_after == ${timestampAfter}`, `display_time_before == ${timestampAfter + SECONDS_PER_DAY}`];
};

export const buildDisplayTimeRangeConditions = (start: dayjs.Dayjs, end: dayjs.Dayjs) => {
  return [`display_time_after == ${Math.floor(start.valueOf() / 1000)}`, `display_time_before == ${Math.floor(end.valueOf() / 1000)}`];
};

export const buildMemoListFilter = ({ filters, fallbackEmotionSearch = false }: BuildMemoListFilterOptions) => {
  const conditions: string[] = [];
  const contentSearch: string[] = [];
  const tagSearch: string[] = [];
  const emotionSearch: string[] = [];

  for (const filter of filters) {
    switch (filter.factor) {
      case "contentSearch":
        contentSearch.push(FILTER_VALUE_QUOTE(filter.value));
        break;
      case "tagSearch":
        tagSearch.push(FILTER_VALUE_QUOTE(filter.value));
        break;
      case "emotionSearch":
        emotionSearch.push(FILTER_VALUE_QUOTE(filter.value));
        break;
      case "pinned":
        conditions.push("pinned == true");
        break;
      case "property.hasLink":
        conditions.push("has_link == true");
        break;
      case "property.hasTaskList":
        conditions.push("has_task_list == true");
        break;
      case "property.hasCode":
        conditions.push("has_code == true");
        break;
      case "displayTime":
        conditions.push(...buildDisplayTimeCondition(filter.value));
        break;
      default:
        break;
    }
  }

  if (contentSearch.length > 0) {
    conditions.push(`content_search == [${contentSearch.join(", ")}]`);
  }
  if (tagSearch.length > 0) {
    conditions.push(`tag_search == [${tagSearch.join(", ")}]`);
  }
  if (emotionSearch.length > 0) {
    const normalizedEmotionSearch = getEmotionSearchValues(emotionSearch.map((value) => JSON.parse(value) as string));
    if (normalizedEmotionSearch.length > 0) {
      conditions.push(`emotion_search == [${normalizedEmotionSearch.map((emotion) => FILTER_VALUE_QUOTE(emotion)).join(", ")}]`);
    }
  } else if (fallbackEmotionSearch) {
    conditions.push(`emotion_search == [${EMOTION_ORDER.map((emotion) => FILTER_VALUE_QUOTE(emotion)).join(", ")}]`);
  }

  return conditions.join(" && ");
};

export type { BuildMemoListFilterOptions };
export type { FilterFactor, MemoFilter };
