import type { models } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ModalPortal } from "../ui/ModalPortal";
import { ProxyImage } from "../ui/ProxyImage";

interface AddGameToCategoryModalProps {
  isOpen: boolean;
  allGames: models.Game[];
  loading?: boolean;
  hasMore?: boolean;
  onSearchChange?: (query: string) => void;
  onLoadMore?: () => void;
  onClose: () => void;
  onAddGame: (gameId: string) => void;
}

export function AddGameToCategoryModal({
  isOpen,
  allGames,
  loading = false,
  hasMore = false,
  onSearchChange,
  onLoadMore,
  onClose,
  onAddGame,
}: AddGameToCategoryModalProps) {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    if (!isOpen) {
      setSearchQuery("");
    }
  }, [isOpen]);

  useEffect(() => {
    onSearchChange?.(searchQuery);
  }, [onSearchChange, searchQuery]);

  if (!isOpen)
    return null;

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="w-full max-w-2xl h-[80vh] rounded-xl bg-white flex flex-col shadow-xl dark:bg-brand-800">
          <div className="p-6 border-b border-brand-200 dark:border-brand-700 flex justify-between items-center">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white">
              {t("category.addGameModal.title")}
            </h3>
            <button
              type="button"
              onClick={onClose}
              className="text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white"
            >
              <div className="i-mdi-close text-xl" />
            </button>
          </div>

          <div className="px-6 pt-4">
            <div className="relative">
              <div className="absolute left-3 top-1/2 -translate-y-1/2 i-mdi-magnify text-brand-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={event => setSearchQuery(event.target.value)}
                placeholder={t("library.searchPlaceholder")}
                className="glass-input w-full rounded-lg border border-brand-200 bg-white py-2 pl-10 pr-3 text-sm text-brand-900 outline-none focus:ring-2 focus:ring-primary-500 dark:border-brand-700 dark:bg-brand-900 dark:text-white"
              />
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-6">
            {allGames.length > 0 ? (
              <>
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
                  {allGames.map(game => (
                    <button
                      key={game.id}
                      type="button"
                      onClick={() => onAddGame(game.id)}
                      className="flex flex-col items-center p-2 rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors text-left group"
                    >
                      <div className="w-full aspect-[3/4] rounded-lg overflow-hidden bg-brand-200 dark:bg-brand-700 mb-2 relative">
                        {game.cover_url ? (
                          <ProxyImage
                            src={game.cover_url}
                            alt={game.name}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <div className="w-full h-full flex items-center justify-center text-brand-400">
                            <div className="i-mdi-image-off text-2xl" />
                          </div>
                        )}
                        <div className="absolute inset-0 bg-black/40 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                          <div className="i-mdi-plus text-white text-3xl" />
                        </div>
                      </div>
                      <span className="text-sm font-medium text-brand-900 dark:text-white line-clamp-2 w-full">
                        {game.name}
                      </span>
                    </button>
                  ))}
                </div>
                {hasMore && (
                  <div className="mt-4 flex justify-center">
                    <button
                      type="button"
                      onClick={onLoadMore}
                      disabled={loading}
                      className="glass-btn-neutral rounded-lg px-4 py-2 text-sm text-brand-700 disabled:opacity-50 dark:text-brand-200"
                    >
                      {loading
                        ? t("common.loading", "加载中...")
                        : t("common.loadMore", "加载更多")}
                    </button>
                  </div>
                )}
              </>
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-brand-500">
                <p>
                  {loading
                    ? t("common.loading", "加载中...")
                    : t("category.addGameModal.noGamesAvailable")}
                </p>
              </div>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
