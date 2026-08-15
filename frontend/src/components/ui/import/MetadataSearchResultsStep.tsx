import type { ReactNode } from "react";
import type { vo } from "../../../../src/bindings/models";
import { Popover, PopoverButton, PopoverPanel } from "@headlessui/react";
import { useCallback, useEffect, useReducer, useRef } from "react";
import { useTranslation } from "react-i18next";
import { BetterEdgeIconButton } from "../better/BetterEdgeIconButton";
import { GameCoverImage } from "../GameCoverImage";
import { sourceLabel } from "./importFlow";

interface MetadataSearchResultsStepProps {
  results: vo.GameMetadataFromWebVO[];
  onSelect: (result: vo.GameMetadataFromWebVO) => void;
  onRemove: (index: number) => void;
  footer: ReactNode;
  disabled?: boolean;
  empty?: ReactNode;
}

interface ResultsScrollState {
  canScrollPrev: boolean;
  canScrollNext: boolean;
}

function replaceScrollState(
  _current: ResultsScrollState,
  next: ResultsScrollState,
) {
  return next;
}

export function MetadataSearchResultsStep({
  results,
  onSelect,
  onRemove,
  footer,
  disabled = false,
  empty,
}: MetadataSearchResultsStepProps) {
  const { t } = useTranslation();
  const resultsScrollerRef = useRef<HTMLDivElement | null>(null);
  const [scrollState, updateResultsScrollState] = useReducer(
    replaceScrollState,
    { canScrollPrev: false, canScrollNext: false },
  );

  const updateScrollState = useCallback(() => {
    const scroller = resultsScrollerRef.current;
    if (!scroller)
      return;

    const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth;
    updateResultsScrollState({
      canScrollPrev: scroller.scrollLeft > 2,
      canScrollNext: scroller.scrollLeft < maxScrollLeft - 2,
    });
  }, []);

  useEffect(() => {
    const scroller = resultsScrollerRef.current;
    if (!scroller)
      return;

    updateScrollState();
    scroller.addEventListener("scroll", updateScrollState, { passive: true });
    window.addEventListener("resize", updateScrollState);

    const resizeObserver = new ResizeObserver(updateScrollState);
    resizeObserver.observe(scroller);

    return () => {
      scroller.removeEventListener("scroll", updateScrollState);
      window.removeEventListener("resize", updateScrollState);
      resizeObserver.disconnect();
    };
  }, [results.length, updateScrollState]);

  const scrollResults = (direction: -1 | 1) => {
    const scroller = resultsScrollerRef.current;
    if (!scroller)
      return;

    scroller.scrollBy({
      behavior: "smooth",
      left: direction * Math.max(scroller.clientWidth - 96, 240),
    });
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-1">
        <p className="text-brand-600 dark:text-brand-300">
          {t("addGameModal.whichResult")}
        </p>
        <Popover className="relative">
          <PopoverButton
            aria-label={t("addGameModal.selectionHelp")}
            className="flex h-7 w-7 items-center justify-center rounded-full text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
          >
            <span
              className="i-mdi-help-circle-outline text-lg"
              aria-hidden="true"
            />
          </PopoverButton>
          <PopoverPanel
            anchor="bottom start"
            className="z-[9999] mt-2 w-80 max-w-[calc(100vw-2rem)] rounded-xl border border-brand-200 bg-white p-4 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90 [--anchor-gap:8px]"
          >
            <h3 className="text-sm font-semibold text-brand-900 dark:text-white">
              {t("addGameModal.selectionHelpTitle")}
            </h3>
            <ol className="mt-3 space-y-2.5 text-sm leading-5 text-brand-600 dark:text-brand-300">
              <li className="flex gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-brand-100 text-xs font-semibold text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                  1
                </span>
                <span>{t("addGameModal.selectionHelpRemove")}</span>
              </li>
              <li className="flex gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-brand-100 text-xs font-semibold text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                  2
                </span>
                <span>{t("addGameModal.selectionHelpUnique")}</span>
              </li>
              <li className="flex gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-brand-100 text-xs font-semibold text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                  3
                </span>
                <span>{t("addGameModal.selectionHelpDefault")}</span>
              </li>
            </ol>
          </PopoverPanel>
        </Popover>
      </div>

      {results.length > 0 ? (
        <div className="relative">
          <div
            ref={resultsScrollerRef}
            className="flex w-full snap-x gap-4 overflow-x-auto p-2 pb-6 pt-2 scroll-smooth [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
          >
            {results.map((item, index) =>
              item.Game ? (
                <div
                  key={`${item.Source}:${item.Game.source_id}`}
                  className="relative w-36 shrink-0 snap-center rounded-xl border border-brand-200 bg-brand-50/50 shadow-sm transition-all hover:border-brand-400 hover:shadow-md dark:border-brand-700 dark:bg-brand-800/50 dark:hover:border-brand-500 sm:w-40"
                >
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => onSelect(item)}
                    className="block w-full cursor-pointer p-3 text-left disabled:cursor-wait disabled:opacity-60"
                  >
                    <div className="aspect-[3/4] w-full overflow-hidden rounded-md bg-brand-200 dark:bg-brand-700">
                      {item.Game.cover_url || item.Game.cover_source_url ? (
                        <GameCoverImage
                          src={
                            item.Game.cover_url || item.Game.cover_source_url
                          }
                          fallbackSrc={item.Game.cover_source_url}
                          alt={item.Game.name}
                          isNSFW={item.Game.is_nsfw}
                          className="h-full w-full"
                          imageClassName="h-full w-full object-cover"
                        />
                      ) : (
                        <div className="flex h-full items-center justify-center text-brand-400">
                          <div className="i-mdi-image-off text-4xl" />
                        </div>
                      )}
                    </div>
                    <h3 className="mt-2 truncate text-sm font-bold text-brand-900 dark:text-white">
                      {item.Game.name}
                    </h3>
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("addGameModal.fromSource", {
                        source: sourceLabel(item.Source, t),
                      })}
                    </p>
                  </button>
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => onRemove(index)}
                    aria-label={t("addGameModal.removeResult", {
                      source: sourceLabel(item.Source, t),
                    })}
                    className="group absolute -right-3 -top-3 z-10 flex h-11 w-11 items-center justify-center rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-error-400 disabled:cursor-wait disabled:opacity-60"
                  >
                    <span
                      className="flex h-8 w-8 items-center justify-center rounded-full border-2 border-white bg-brand-300/85 text-white shadow-md backdrop-blur-sm transition-colors group-hover:bg-error-600 dark:border-brand-700 dark:bg-brand-900/90 dark:group-hover:bg-error-600"
                      aria-hidden="true"
                    >
                      <span className="i-mdi-close text-lg" />
                    </span>
                  </button>
                </div>
              ) : null,
            )}
          </div>

          {scrollState.canScrollPrev ? (
            <BetterEdgeIconButton
              placement="left"
              icon="i-mdi-chevron-left"
              onClick={() => scrollResults(-1)}
              aria-label={t(
                "addGameModal.scrollResultsPrev",
                "向前查看更多结果",
              )}
              className="absolute left-0 top-1/2 z-10 -translate-y-1/2"
            />
          ) : null}

          {scrollState.canScrollNext ? (
            <BetterEdgeIconButton
              placement="right"
              icon="i-mdi-chevron-right"
              onClick={() => scrollResults(1)}
              aria-label={t(
                "addGameModal.scrollResultsNext",
                "向后查看更多结果",
              )}
              className="absolute right-0 top-1/2 z-10 -translate-y-1/2"
            />
          ) : null}
        </div>
      ) : (
        empty
      )}

      {footer}
    </div>
  );
}
