import { uniqueId } from "lodash-es";
import { makeAutoObservable } from "mobx";
import { memoServiceClient } from "@/grpcweb";
import { Direction, State } from "@/types/proto/api/v1/common";
import { CreateMemoRequest, ListMemosRequest, Memo } from "@/types/proto/api/v1/memo_service";
import { getErrorMessage } from "@/utils/error";

type FetchMemosRequest = Pick<ListMemosRequest, "parent" | "pageSize" | "pageToken" | "state" | "direction" | "filter" | "oldFilter">;

class LocalState {
  stateId: string = uniqueId();
  memoMapByName: Record<string, Memo> = {};
  currentRequest: AbortController | null = null;
  currentRequestKey: string = "";
  currentRequestPromise: Promise<{ memos: Memo[]; nextPageToken: string } | void> | null = null;
  inflightMemoByName: Record<string, Promise<Memo>> = {};
  lastError: string = "";

  constructor() {
    makeAutoObservable(this);
  }

  setPartial(partial: Partial<LocalState>) {
    Object.assign(this, partial);
  }

  updateStateId() {
    this.stateId = uniqueId();
  }

  get memos() {
    return Object.values(this.memoMapByName);
  }

  get size() {
    return Object.keys(this.memoMapByName).length;
  }
}

const memoStore = (() => {
  const state = new LocalState();
  const normalizeFetchRequest = (request: Partial<FetchMemosRequest>): FetchMemosRequest => ({
    parent: request.parent ?? "",
    pageSize: request.pageSize ?? 0,
    pageToken: request.pageToken ?? "",
    state: request.state ?? State.STATE_UNSPECIFIED,
    direction: request.direction ?? Direction.DIRECTION_UNSPECIFIED,
    filter: request.filter ?? "",
    oldFilter: request.oldFilter ?? "",
  });

  const buildRequestKey = (request: FetchMemosRequest) =>
    JSON.stringify({
      parent: request.parent,
      pageSize: request.pageSize,
      pageToken: request.pageToken,
      state: request.state,
      direction: request.direction,
      filter: request.filter,
      oldFilter: request.oldFilter,
    });

  const fetchMemos = async (request: Partial<FetchMemosRequest>) => {
    const normalizedRequest = normalizeFetchRequest(request);
    const requestKey = buildRequestKey(normalizedRequest);
    if (state.currentRequest && state.currentRequestKey === requestKey && state.currentRequestPromise) {
      return state.currentRequestPromise;
    }

    if (state.currentRequest) {
      state.currentRequest.abort();
    }

    const controller = new AbortController();
    state.setPartial({ currentRequest: controller, currentRequestKey: requestKey, lastError: "" });

    const requestPromise = memoServiceClient
      .listMemos(
        {
          ...normalizedRequest,
        },
        { signal: controller.signal },
      )
      .then(({ memos, nextPageToken }) => {
        if (!controller.signal.aborted) {
          const memoMap = normalizedRequest.pageToken ? { ...state.memoMapByName } : {};
          for (const memo of memos) {
            memoMap[memo.name] = memo;
          }
          state.setPartial({
            stateId: uniqueId(),
            memoMapByName: memoMap,
            lastError: "",
          });
          return { memos, nextPageToken };
        }
      })
      .catch((error: any) => {
        if (error.name === "AbortError") {
          return;
        }
        state.setPartial({ lastError: getErrorMessage(error) });
        throw error;
      })
      .finally(() => {
        if (state.currentRequest === controller) {
          state.setPartial({ currentRequest: null, currentRequestKey: "", currentRequestPromise: null });
        }
      });

    state.setPartial({ currentRequestPromise: requestPromise });
    return requestPromise;
  };

  const getOrFetchMemoByName = async (name: string, options?: { skipCache?: boolean; skipStore?: boolean }) => {
    const memoCache = state.memoMapByName[name];
    if (memoCache && !options?.skipCache) {
      return memoCache;
    }
    const inflight = state.inflightMemoByName[name];
    if (inflight) {
      return inflight;
    }

    const request = memoServiceClient
      .getMemo({
        name,
      })
      .then((memo) => {
        if (!options?.skipStore) {
          const memoMap = { ...state.memoMapByName };
          memoMap[name] = memo;
          state.setPartial({
            stateId: uniqueId(),
            memoMapByName: memoMap,
          });
        }
        return memo;
      })
      .finally(() => {
        const inflightMap = { ...state.inflightMemoByName };
        delete inflightMap[name];
        state.setPartial({ inflightMemoByName: inflightMap });
      });

    state.setPartial({
      inflightMemoByName: {
        ...state.inflightMemoByName,
        [name]: request,
      },
    });

    return request;
  };

  const getMemoByName = (name: string) => {
    return state.memoMapByName[name];
  };

  const createMemo = async (request: CreateMemoRequest) => {
    const memo = await memoServiceClient.createMemo(request);
    const memoMap = { ...state.memoMapByName };
    memoMap[memo.name] = memo;
    state.setPartial({
      stateId: uniqueId(),
      memoMapByName: memoMap,
    });
    return memo;
  };

  const updateMemo = async (update: Partial<Memo>, updateMask: string[]) => {
    const memo = await memoServiceClient.updateMemo({
      memo: update,
      updateMask,
    });

    const memoMap = { ...state.memoMapByName };
    memoMap[memo.name] = memo;
    state.setPartial({
      stateId: uniqueId(),
      memoMapByName: memoMap,
    });
    return memo;
  };

  const deleteMemo = async (name: string) => {
    await memoServiceClient.deleteMemo({
      name,
    });

    const memoMap = { ...state.memoMapByName };
    delete memoMap[name];
    state.setPartial({
      stateId: uniqueId(),
      memoMapByName: memoMap,
    });
  };

  return {
    state,
    fetchMemos,
    getOrFetchMemoByName,
    getMemoByName,
    createMemo,
    updateMemo,
    deleteMemo,
  };
})();

export default memoStore;
