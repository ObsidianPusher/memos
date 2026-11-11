import dayjs from "dayjs";
import { observer } from "mobx-react-lite";
import { useCallback, useMemo } from "react";
import MemoView from "@/components/MemoView";
import PagedMemoList from "@/components/PagedMemoList";
import useCurrentUser from "@/hooks/useCurrentUser";
import { viewStore, userStore } from "@/store/v2";
import memoFilterStore from "@/store/v2/memoFilter";
import { Direction, State } from "@/types/proto/api/v1/common";
import { Memo } from "@/types/proto/api/v1/memo_service";
import { buildMemoListFilter } from "@/utils/memoListFilter";

const Home = observer(() => {
  const user = useCurrentUser();
  const orderByTimeAsc = viewStore.state.orderByTimeAsc;
  const selectedShortcut = userStore.state.shortcuts.find((shortcut) => shortcut.id === memoFilterStore.shortcut);

  const memoListFilter = useMemo(() => buildMemoListFilter({ filters: memoFilterStore.filters }), [memoFilterStore.filters]);

  const renderMemo = useCallback((memo: Memo) => <MemoView memo={memo} showVisibility showPinned compact />, []);

  const sortMemos = useCallback(
    (memos: Memo[]) =>
      memos
        .filter((memo) => memo.state === State.NORMAL)
        .sort((a, b) => {
          const pinnedDiff = Number(b.pinned) - Number(a.pinned);
          if (pinnedDiff !== 0) {
            return pinnedDiff;
          }

          return orderByTimeAsc
            ? dayjs(a.displayTime).unix() - dayjs(b.displayTime).unix()
            : dayjs(b.displayTime).unix() - dayjs(a.displayTime).unix();
        }),
    [orderByTimeAsc],
  );

  return (
    <PagedMemoList
      renderer={renderMemo}
      listSort={sortMemos}
      owner={user.name}
      direction={orderByTimeAsc ? Direction.ASC : Direction.DESC}
      filter={selectedShortcut?.filter || ""}
      oldFilter={memoListFilter}
    />
  );
});

export default Home;
