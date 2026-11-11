import { Button } from "@usememos/mui";
import { ArrowUpIcon, LoaderIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { matchPath } from "react-router-dom";
import PullToRefresh from "react-simple-pull-to-refresh";
import { DEFAULT_LIST_MEMOS_PAGE_SIZE } from "@/helpers/consts";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";
import { Routes } from "@/router/routes";
import { memoStore, viewStore } from "@/store/v2";
import { Direction, State } from "@/types/proto/api/v1/common";
import { Memo } from "@/types/proto/api/v1/memo_service";
import { getErrorMessage } from "@/utils/error";
import { useTranslate } from "@/utils/i18n";
import Empty from "../Empty";
import MasonryView from "../MasonryView";
import MemoEditor from "../MemoEditorLazy";

interface Props {
  renderer: (memo: Memo) => JSX.Element;
  listSort?: (list: Memo[]) => Memo[];
  owner?: string;
  state?: State;
  direction?: Direction;
  filter?: string;
  oldFilter?: string;
  pageSize?: number;
}

const PagedMemoList = observer((props: Props) => {
  const t = useTranslate();
  const { md } = useResponsiveWidth();

  // Simplified state management - separate state variables for clarity
  const [isRequesting, setIsRequesting] = useState(true);
  const [nextPageToken, setNextPageToken] = useState("");

  const isFetchingRef = useRef(false);
  const bottomSentinelRef = useRef<HTMLDivElement | null>(null);

  const memoStateId = memoStore.state.stateId;

  // Apply custom sorting only when memo data actually changes.
  const sortedMemoList = useMemo(() => {
    const currentMemoList = memoStore.state.memos;
    return props.listSort ? props.listSort(currentMemoList) : currentMemoList;
  }, [memoStateId, props.listSort]);

  // Show memo editor only on the root route
  const showMemoEditor = Boolean(matchPath(Routes.ROOT, window.location.pathname));

  // Fetch more memos with pagination support
  const fetchMoreMemos = useCallback(
    async (pageToken: string) => {
      if (isFetchingRef.current) {
        return;
      }
      isFetchingRef.current = true;
      setIsRequesting(true);

      try {
        const response = await memoStore.fetchMemos({
          parent: props.owner || "",
          state: props.state || State.NORMAL,
          direction: props.direction || Direction.DESC,
          filter: props.filter || "",
          oldFilter: props.oldFilter || "",
          pageSize: props.pageSize || DEFAULT_LIST_MEMOS_PAGE_SIZE,
          pageToken,
        });

        setNextPageToken(response?.nextPageToken || "");
        return response;
      } catch (error) {
        console.error("failed to fetch memos", getErrorMessage(error));
      } finally {
        isFetchingRef.current = false;
        setIsRequesting(false);
      }
    },
    [props.direction, props.filter, props.oldFilter, props.owner, props.pageSize, props.state],
  );

  // Helper function to check if page has enough content to be scrollable
  const isPageScrollable = () => {
    const documentHeight = Math.max(document.body.scrollHeight, document.documentElement.scrollHeight);
    return documentHeight > window.innerHeight + 100; // 100px buffer for safe measure
  };

  const checkAndFetchIfNeeded = useCallback(
    async (pageToken: string) => {
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));

      let token = pageToken;
      while (token && !isFetchingRef.current && sortedMemoList.length > 0 && !isPageScrollable()) {
        const response = await fetchMoreMemos(token);
        token = response?.nextPageToken || "";
        if (!token || isPageScrollable()) {
          break;
        }
      }
    },
    [fetchMoreMemos, sortedMemoList.length],
  );

  // Refresh the entire memo list from the beginning
  const refreshList = useCallback(async () => {
    memoStore.state.updateStateId();
    setNextPageToken("");
    try {
      const response = await fetchMoreMemos("");
      if (response?.nextPageToken) {
        void checkAndFetchIfNeeded(response.nextPageToken);
      }
    } catch (error) {
      console.error("failed to refresh memo list", getErrorMessage(error));
    }
  }, [checkAndFetchIfNeeded, fetchMoreMemos]);

  // Initial load and reload when props change
  useEffect(() => {
    void refreshList();
  }, [props.owner, props.state, props.direction, props.filter, props.oldFilter, props.pageSize, refreshList]);

  // Auto-fetch more content when list changes and page isn't full
  useEffect(() => {
    if (!isRequesting && nextPageToken && sortedMemoList.length > 0) {
      void checkAndFetchIfNeeded(nextPageToken);
    }
  }, [sortedMemoList.length, isRequesting, nextPageToken, checkAndFetchIfNeeded]);

  // Infinite scroll: fetch more when the bottom sentinel enters the viewport.
  useEffect(() => {
    if (!nextPageToken || !bottomSentinelRef.current) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const [entry] = entries;
        if (entry?.isIntersecting && !isRequesting && !isFetchingRef.current) {
          void fetchMoreMemos(nextPageToken);
        }
      },
      {
        root: null,
        rootMargin: "300px 0px",
        threshold: 0,
      },
    );

    observer.observe(bottomSentinelRef.current);
    return () => observer.disconnect();
  }, [fetchMoreMemos, isRequesting, nextPageToken]);

  const children = (
    <div className="flex flex-col justify-start items-start w-full max-w-full">
      <MasonryView
        memoList={sortedMemoList}
        renderer={props.renderer}
        prefixElement={showMemoEditor ? <MemoEditor className="mb-2" cacheKey="home-memo-editor" /> : undefined}
        listMode={viewStore.state.layout === "LIST"}
      />

      {/* Loading indicator */}
      {isRequesting && (
        <div className="w-full flex flex-row justify-center items-center my-4">
          <LoaderIcon className="animate-spin text-zinc-500" />
        </div>
      )}

      <div ref={bottomSentinelRef} className="h-px w-full" />

      {!isRequesting && memoStore.state.lastError && (
        <div className="w-full my-3 rounded-lg border border-red-200/80 dark:border-red-500/30 bg-red-50/70 dark:bg-red-500/10 px-3 py-2 text-sm text-red-700 dark:text-red-200">
          {memoStore.state.lastError}
        </div>
      )}

      {/* Empty state or back-to-top button */}
      {!isRequesting && (
        <>
          {!nextPageToken && sortedMemoList.length === 0 ? (
            <div className="w-full mt-12 mb-8 flex flex-col justify-center items-center italic">
              <Empty />
              <p className="mt-2 text-gray-600 dark:text-gray-400">{t("message.no-data")}</p>
            </div>
          ) : (
            <div className="w-full opacity-70 flex flex-row justify-center items-center my-4">
              <BackToTop />
            </div>
          )}
        </>
      )}
    </div>
  );

  if (md) {
    return children;
  }

  return (
    <PullToRefresh
      onRefresh={() => refreshList()}
      pullingContent={
        <div className="w-full flex flex-row justify-center items-center my-4">
          <LoaderIcon className="opacity-60" />
        </div>
      }
      refreshingContent={
        <div className="w-full flex flex-row justify-center items-center my-4">
          <LoaderIcon className="animate-spin" />
        </div>
      }
    >
      {children}
    </PullToRefresh>
  );
});

const BackToTop = () => {
  const t = useTranslate();
  const [isVisible, setIsVisible] = useState(false);
  const frameRef = useRef<number | null>(null);

  useEffect(() => {
    const handleScroll = () => {
      if (frameRef.current !== null) {
        return;
      }

      frameRef.current = window.requestAnimationFrame(() => {
        frameRef.current = null;
        const shouldShow = window.scrollY > 400;
        setIsVisible((prevVisible) => (prevVisible === shouldShow ? prevVisible : shouldShow));
      });
    };

    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      window.removeEventListener("scroll", handleScroll);
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
      }
    };
  }, []);

  const scrollToTop = () => {
    window.scrollTo({
      top: 0,
      behavior: "smooth",
    });
  };

  // Don't render if not visible
  if (!isVisible) {
    return null;
  }

  return (
    <Button variant="plain" onClick={scrollToTop}>
      {t("router.back-to-top")}
      <ArrowUpIcon className="ml-1 w-4 h-auto" />
    </Button>
  );
};

export default PagedMemoList;
