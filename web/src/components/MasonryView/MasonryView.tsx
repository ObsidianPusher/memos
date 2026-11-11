import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Memo } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";

interface Props {
  memoList: Memo[];
  renderer: (memo: Memo) => JSX.Element;
  prefixElement?: JSX.Element;
  listMode?: boolean;
}

interface MemoItemProps {
  memo: Memo;
  renderer: (memo: Memo) => JSX.Element;
  onHeightChange: (memoName: string, height: number) => void;
}

// Minimum width required to show more than one column
const MINIMUM_MEMO_VIEWPORT_WIDTH = 512;

const MemoItem = ({ memo, renderer, onHeightChange }: MemoItemProps) => {
  const itemRef = useRef<HTMLDivElement>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const lastHeightRef = useRef<number>(0);

  useEffect(() => {
    if (!itemRef.current) return;

    const measureHeight = () => {
      if (itemRef.current) {
        const height = itemRef.current.offsetHeight;
        if (height === lastHeightRef.current) {
          return;
        }
        lastHeightRef.current = height;
        onHeightChange(memo.name, height);
      }
    };

    measureHeight();

    // Set up ResizeObserver to track dynamic content changes (images, expanded text, etc.)
    resizeObserverRef.current = new ResizeObserver(measureHeight);
    resizeObserverRef.current.observe(itemRef.current);

    return () => {
      resizeObserverRef.current?.disconnect();
    };
  }, [memo.name, onHeightChange]);

  return <div ref={itemRef}>{renderer(memo)}</div>;
};

/**
 * Algorithm to distribute memos into columns based on height for balanced layout
 * Uses greedy approach: always place next memo in the shortest column
 */
const distributeMemosToColumns = (
  memos: Memo[],
  columns: number,
  itemHeights: Map<string, number>,
  prefixElementHeight: number = 0,
): { distribution: number[][]; columnHeights: number[] } => {
  // List mode: all memos in single column
  if (columns === 1) {
    return {
      distribution: [Array.from({ length: memos.length }, (_, i) => i)],
      columnHeights: [prefixElementHeight + memos.reduce((sum, memo) => sum + (itemHeights.get(memo.name) || 0), 0)],
    };
  }

  const distribution: number[][] = Array.from({ length: columns }, () => []);
  const columnHeights: number[] = Array(columns).fill(0);

  // Add prefix element height to first column
  if (prefixElementHeight > 0) {
    columnHeights[0] = prefixElementHeight;
  }

  // Distribute each memo to the shortest column
  memos.forEach((memo, index) => {
    const height = itemHeights.get(memo.name) || 0;
    let shortestColumnIndex = 0;
    for (let i = 1; i < columnHeights.length; i += 1) {
      if (columnHeights[i] < columnHeights[shortestColumnIndex]) {
        shortestColumnIndex = i;
      }
    }

    distribution[shortestColumnIndex].push(index);
    columnHeights[shortestColumnIndex] += height;
  });

  return { distribution, columnHeights };
};

const MasonryView = (props: Props) => {
  const [columns, setColumns] = useState(1);
  const [itemHeights, setItemHeights] = useState<Map<string, number>>(new Map());
  const [prefixHeight, setPrefixHeight] = useState(0);

  const containerRef = useRef<HTMLDivElement>(null);
  const prefixElementRef = useRef<HTMLDivElement>(null);
  const containerResizeObserverRef = useRef<ResizeObserver | null>(null);
  const prefixResizeObserverRef = useRef<ResizeObserver | null>(null);

  // Calculate optimal number of columns based on container width
  const calculateColumns = useCallback(() => {
    if (!containerRef.current || props.listMode) return 1;

    const containerWidth = containerRef.current.offsetWidth;
    const scale = containerWidth / MINIMUM_MEMO_VIEWPORT_WIDTH;
    return scale >= 2 ? Math.round(scale) : 1;
  }, [props.listMode]);

  const distribution = useMemo(
    () => distributeMemosToColumns(props.memoList, columns, itemHeights, prefixHeight).distribution,
    [props.memoList, columns, itemHeights, prefixHeight],
  );

  // Handle height changes from individual memo items
  const handleHeightChange = useCallback((memoName: string, height: number) => {
    setItemHeights((prevHeights) => {
      if (prevHeights.get(memoName) === height) {
        return prevHeights;
      }
      const newItemHeights = new Map(prevHeights);
      newItemHeights.set(memoName, height);
      return newItemHeights;
    });
  }, []);

  useEffect(() => {
    if (!containerRef.current) return;

    const updateColumns = () => {
      const newColumns = calculateColumns();
      setColumns((prevColumns) => (prevColumns === newColumns ? prevColumns : newColumns));
    };

    updateColumns();
    containerResizeObserverRef.current?.disconnect();
    containerResizeObserverRef.current = new ResizeObserver(() => {
      updateColumns();
    });
    containerResizeObserverRef.current.observe(containerRef.current);

    return () => {
      containerResizeObserverRef.current?.disconnect();
      containerResizeObserverRef.current = null;
    };
  }, [calculateColumns]);

  useEffect(() => {
    if (!prefixElementRef.current) {
      setPrefixHeight(0);
      return;
    }

    const updatePrefixHeight = () => {
      const height = prefixElementRef.current?.offsetHeight || 0;
      setPrefixHeight((prevHeight) => (prevHeight === height ? prevHeight : height));
    };

    updatePrefixHeight();
    prefixResizeObserverRef.current?.disconnect();
    prefixResizeObserverRef.current = new ResizeObserver(() => {
      updatePrefixHeight();
    });
    prefixResizeObserverRef.current.observe(prefixElementRef.current);

    return () => {
      prefixResizeObserverRef.current?.disconnect();
      prefixResizeObserverRef.current = null;
    };
  }, [props.prefixElement]);

  const isListMode = props.listMode || columns === 1;

  return (
    <div ref={containerRef} className={cn("w-full grid gap-2")} style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}>
      {isListMode ? (
        <div className="min-w-0 mx-auto w-full max-w-2xl">
          {props.prefixElement && <div ref={prefixElementRef}>{props.prefixElement}</div>}
          {props.memoList.map((memo) => (
            <div key={memo.name}>{props.renderer(memo)}</div>
          ))}
        </div>
      ) : (
        Array.from({ length: columns }).map((_, columnIndex) => (
          <div key={columnIndex} className="min-w-0 mx-auto w-full max-w-2xl">
            {props.prefixElement && columnIndex === 0 && <div ref={prefixElementRef}>{props.prefixElement}</div>}

            {distribution[columnIndex]?.map((memoIndex) => {
              const memo = props.memoList[memoIndex];
              return memo ? <MemoItem key={memo.name} memo={memo} renderer={props.renderer} onHeightChange={handleHeightChange} /> : null;
            })}
          </div>
        ))
      )}
    </div>
  );
};

export default MasonryView;
