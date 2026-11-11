import { memoServiceClient, API_ENDPOINT } from "@/grpcweb";
import { Emotion, ListMemosRequest, Memo } from "@/types/proto/api/v1/memo_service";

type ListMemosOptions = Partial<Pick<ListMemosRequest, "parent" | "pageSize" | "state" | "direction" | "filter" | "oldFilter">> & {
  signal?: AbortSignal;
};

const DEFAULT_PAGE_SIZE = 100;

export const listMemosPage = async (options: ListMemosOptions & Pick<ListMemosRequest, "pageToken">) => {
  return memoServiceClient.listMemos(
    {
      parent: options.parent ?? "",
      pageSize: options.pageSize ?? DEFAULT_PAGE_SIZE,
      pageToken: options.pageToken ?? "",
      state: options.state,
      direction: options.direction,
      filter: options.filter ?? "",
      oldFilter: options.oldFilter ?? "",
    },
    options.signal ? { signal: options.signal } : undefined,
  );
};

export const listAllMemos = async (options: ListMemosOptions): Promise<Memo[]> => {
  const memos: Memo[] = [];
  let pageToken = "";

  do {
    const response = await listMemosPage({
      ...options,
      pageToken,
    });
    memos.push(...response.memos);
    pageToken = response.nextPageToken;
  } while (pageToken);

  return memos;
};

export interface EmotionReportPoint {
  labelLines: string[];
  value: number;
  count: number;
  dominantEmotion: Emotion;
}

export interface EmotionReportBreakdownItem {
  emotion: Emotion;
  count: number;
  ratio: number;
}

export interface EmotionReportResponse {
  total: number;
  averageScore: number;
  dominantEmotion: Emotion;
  breakdown: EmotionReportBreakdownItem[];
  series: EmotionReportPoint[];
}

interface EmotionReportRequest {
  startTime: string;
  endTime: string;
  granularity: "week" | "month";
  filter?: string;
  oldFilter?: string;
  signal?: AbortSignal;
}

export const fetchEmotionReport = async (request: EmotionReportRequest): Promise<EmotionReportResponse> => {
  const params = new URLSearchParams();
  params.set("start_time", request.startTime);
  params.set("end_time", request.endTime);
  params.set("granularity", request.granularity);
  if (request.filter) {
    params.set("filter", request.filter);
  }
  if (request.oldFilter) {
    params.set("old_filter", request.oldFilter);
  }

  const response = await fetch(`${API_ENDPOINT}/api/v1/emotions/report?${params.toString()}`, {
    credentials: "include",
    signal: request.signal,
  });

  if (!response.ok) {
    let errorMessage = "failed to fetch emotion report";
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) {
        errorMessage = payload.error;
      }
    } catch {
      // Fallback to the default error message.
    }
    throw new Error(errorMessage);
  }

  return (await response.json()) as EmotionReportResponse;
};
