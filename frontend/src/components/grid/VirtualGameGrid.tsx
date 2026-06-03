import type { ReactNode } from "react";
import type { models } from "../../../wailsjs/go/models";
import { useElementScrollRestoration } from "@tanstack/react-router";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { findScrollParent } from "../../utils/scroll";
import { GameCard } from "../card/GameCard";

const DEFAULT_ROOT_FONT_SIZE = 16;
const CARD_MIN_WIDTH_REM = 8.75;
const GRID_GAP_REM = 0.75;
const CARD_IMAGE_ASPECT_RATIO = 3.6 / 3;
const CARD_META_HEIGHT_REM = 3.5;

function getRootFontSize() {
  if (typeof window === "undefined") {
    return DEFAULT_ROOT_FONT_SIZE;
  }

  const fontSize = Number.parseFloat(
    window.getComputedStyle(document.documentElement).fontSize,
  );
  return Number.isFinite(fontSize) && fontSize > 0
    ? fontSize
    : DEFAULT_ROOT_FONT_SIZE;
}

interface VirtualGameGridProps {
  gamesByIndex: ReadonlyMap<number, models.Game>;
  scrollRestorationId: string;
  totalItems: number;
  visibleRangeResetKey?: string;
  searchQuery?: string;
  selectionMode?: boolean;
  selectedGameIds?: Set<string>;
  onSelectChange?: (gameId: string, selected: boolean) => void;
  onVisibleRangeChange?: (startIndex: number, endIndex: number) => void;
  renderOverlay?: (game: models.Game) => ReactNode;
}

interface GameGridCellProps {
  game: models.Game;
  renderOverlay?: (game: models.Game) => ReactNode;
  searchQuery: string;
  selected: boolean;
  selectionMode: boolean;
  onSelectChange: (gameId: string, selected: boolean) => void;
}

const GameGridCell = memo(
  ({
    game,
    renderOverlay,
    searchQuery,
    selected,
    selectionMode,
    onSelectChange,
  }: GameGridCellProps) => {
    const handleSelectChange = useCallback(
      (nextSelected: boolean) => {
        onSelectChange(game.id, nextSelected);
      },
      [game.id, onSelectChange],
    );

    return (
      <div className="relative group">
        <GameCard
          game={game}
          searchQuery={searchQuery}
          selected={selected}
          selectionMode={selectionMode}
          onSelectChange={handleSelectChange}
        />
        {renderOverlay?.(game)}
      </div>
    );
  },
);

const GameCardPlaceholder = memo(() => (
  <div className="glass-card pointer-events-none flex w-full animate-pulse flex-col overflow-hidden rounded-xl border border-brand-100 bg-white shadow-sm dark:border-brand-700 dark:bg-brand-800">
    <div className="relative aspect-[3/3.6] w-full bg-brand-200/80 dark:bg-brand-700/80" />
    <div className="space-y-1 px-2 pb-2 pt-1">
      <div className="h-4 w-4/5 rounded bg-brand-200 dark:bg-brand-700" />
      <div className="h-3 w-3/5 rounded bg-brand-200/80 dark:bg-brand-700/80" />
    </div>
  </div>
));

export function VirtualGameGrid({
  gamesByIndex,
  scrollRestorationId,
  totalItems,
  visibleRangeResetKey,
  searchQuery = "",
  selectionMode = false,
  selectedGameIds,
  onSelectChange,
  onVisibleRangeChange,
  renderOverlay,
}: VirtualGameGridProps) {
  const measureRef = useRef<HTMLDivElement | null>(null);
  const lastVisibleRangeRef = useRef("");
  const [scrollElement, setScrollElement] = useState<HTMLElement | null>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const [rootFontSize, setRootFontSize] = useState(getRootFontSize);
  const [scrollMargin, setScrollMargin] = useState(0);
  const scrollEntry = useElementScrollRestoration({
    id: scrollRestorationId,
  });

  useLayoutEffect(() => {
    const element = measureRef.current;
    if (!element) {
      return;
    }

    const updateLayout = () => {
      const nextScrollElement = findScrollParent(element);
      setScrollElement(nextScrollElement);
      setContainerWidth(element.clientWidth);
      setRootFontSize(getRootFontSize());
      setScrollMargin(() => {
        if (!nextScrollElement) {
          return 0;
        }

        const elementRect = element.getBoundingClientRect();
        const scrollRect = nextScrollElement.getBoundingClientRect();
        return elementRect.top - scrollRect.top + nextScrollElement.scrollTop;
      });
    };

    updateLayout();
    const observer = new ResizeObserver(updateLayout);
    observer.observe(element);
    if (element.parentElement) {
      observer.observe(element.parentElement);
    }
    window.addEventListener("resize", updateLayout);
    window.addEventListener("app-zoom-change", updateLayout);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateLayout);
      window.removeEventListener("app-zoom-change", updateLayout);
    };
  }, []);

  const cardMinWidth = CARD_MIN_WIDTH_REM * rootFontSize;
  const gridGap = GRID_GAP_REM * rootFontSize;
  const cardMetaHeight = CARD_META_HEIGHT_REM * rootFontSize;
  const columnCount = Math.max(
    1,
    Math.floor((containerWidth + gridGap) / (cardMinWidth + gridGap)),
  );
  const cardWidth
    = columnCount > 0
      ? (containerWidth - gridGap * (columnCount - 1)) / columnCount
      : cardMinWidth;
  const rowHeight = Math.ceil(
    cardWidth * CARD_IMAGE_ASPECT_RATIO + cardMetaHeight,
  );
  const rowCount = Math.ceil(totalItems / columnCount);

  const virtualizer = useVirtualizer({
    count: rowCount,
    getScrollElement: () => scrollElement,
    estimateSize: () => rowHeight,
    initialOffset: scrollEntry?.scrollY,
    overscan: 2,
    scrollMargin,
  });

  const virtualItems = virtualizer.getVirtualItems();

  useEffect(() => {
    virtualizer.measure();
  }, [columnCount, rowHeight, virtualizer]);

  useEffect(() => {
    lastVisibleRangeRef.current = "";
  }, [visibleRangeResetKey]);

  useEffect(() => {
    const first = virtualItems[0];
    const last = virtualItems.at(-1);
    if (!first || !last || totalItems === 0) {
      return;
    }

    const startIndex = Math.max(0, first.index * columnCount);
    const endIndex = Math.min(
      totalItems - 1,
      (last.index + 1) * columnCount - 1,
    );
    const rangeKey = `${startIndex}:${endIndex}`;
    if (lastVisibleRangeRef.current !== rangeKey) {
      lastVisibleRangeRef.current = rangeKey;
      onVisibleRangeChange?.(startIndex, endIndex);
    }
  }, [
    columnCount,
    onVisibleRangeChange,
    totalItems,
    virtualItems,
    visibleRangeResetKey,
  ]);

  const handleSelectChange = useCallback(
    (gameId: string, selected: boolean) => {
      onSelectChange?.(gameId, selected);
    },
    [onSelectChange],
  );

  const gridTemplateColumns = useMemo(
    () => `repeat(${columnCount}, minmax(0, 1fr))`,
    [columnCount],
  );

  return (
    <div ref={measureRef} className="w-full">
      <div
        className="relative w-full"
        style={{ height: virtualizer.getTotalSize() }}
      >
        {virtualItems.map((virtualRow) => {
          const startIndex = virtualRow.index * columnCount;
          const cells = Array.from(
            { length: columnCount },
            (_, columnIndex) => {
              const itemIndex = startIndex + columnIndex;
              if (itemIndex >= totalItems) {
                return null;
              }

              const game = gamesByIndex.get(itemIndex);
              if (!game) {
                return (
                  <div
                    key={`placeholder-${itemIndex}`}
                    className="relative group"
                  >
                    <GameCardPlaceholder />
                  </div>
                );
              }

              return (
                <GameGridCell
                  key={game.id}
                  game={game}
                  renderOverlay={renderOverlay}
                  searchQuery={searchQuery}
                  selected={selectedGameIds?.has(game.id) ?? false}
                  selectionMode={selectionMode}
                  onSelectChange={handleSelectChange}
                />
              );
            },
          );
          return (
            <div
              key={virtualRow.key}
              className="absolute left-0 top-0 grid w-full gap-3"
              style={{
                gridTemplateColumns,
                transform: `translateY(${
                  virtualRow.start - virtualizer.options.scrollMargin
                }px)`,
              }}
            >
              {cells}
            </div>
          );
        })}
      </div>
    </div>
  );
}
