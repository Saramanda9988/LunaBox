import type { models } from "../../../wailsjs/go/models";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { GameCard } from "../card/GameCard";

const CARD_MIN_WIDTH = 140;
const GRID_GAP = 12;
const ROW_HEIGHT = 220;

function findScrollParent(element: HTMLElement | null): HTMLElement | null {
  let current = element?.parentElement ?? null;
  while (current) {
    const style = window.getComputedStyle(current);
    if (/auto|scroll|overlay/.test(style.overflowY)) {
      return current;
    }
    current = current.parentElement;
  }
  return null;
}

interface VirtualGameGridProps {
  games: models.Game[];
  searchQuery?: string;
  selectionMode?: boolean;
  selectedGameIds?: Set<string>;
  onSelectChange?: (gameId: string, selected: boolean) => void;
  onNearEnd?: () => void;
  renderOverlay?: (game: models.Game) => React.ReactNode;
}

export function VirtualGameGrid({
  games,
  searchQuery = "",
  selectionMode = false,
  selectedGameIds,
  onSelectChange,
  onNearEnd,
  renderOverlay,
}: VirtualGameGridProps) {
  const measureRef = useRef<HTMLDivElement | null>(null);
  const [scrollElement, setScrollElement] = useState<HTMLElement | null>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    const element = measureRef.current;
    if (!element) {
      return;
    }
    const updateWidth = () => setContainerWidth(element.clientWidth);
    updateWidth();
    setScrollElement(findScrollParent(element));
    const observer = new ResizeObserver(updateWidth);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const columnCount = Math.max(
    1,
    Math.floor((containerWidth + GRID_GAP) / (CARD_MIN_WIDTH + GRID_GAP)),
  );
  const rowCount = Math.ceil(games.length / columnCount);

  const virtualizer = useVirtualizer({
    count: rowCount,
    getScrollElement: () => scrollElement,
    estimateSize: () => ROW_HEIGHT,
    overscan: 4,
  });

  const virtualItems = virtualizer.getVirtualItems();

  useEffect(() => {
    const last = virtualItems.at(-1);
    if (!last || games.length === 0) {
      return;
    }
    if ((last.index + 2) * columnCount >= games.length) {
      onNearEnd?.();
    }
  }, [columnCount, games.length, onNearEnd, virtualItems]);

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
          const rowGames = games.slice(startIndex, startIndex + columnCount);
          return (
            <div
              key={virtualRow.key}
              className="absolute left-0 top-0 grid w-full gap-3"
              style={{
                gridTemplateColumns,
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              {rowGames.map(game => (
                <div key={game.id} className="relative group">
                  <GameCard
                    game={game}
                    searchQuery={searchQuery}
                    selectionMode={selectionMode}
                    selected={selectedGameIds?.has(game.id) ?? false}
                    onSelectChange={selected =>
                      handleSelectChange(game.id, selected)}
                  />
                  {renderOverlay?.(game)}
                </div>
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
