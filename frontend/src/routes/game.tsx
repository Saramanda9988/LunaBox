import type { models, vo } from "../../wailsjs/go/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
import { AddGameToCategory, GetCategories, GetCategoriesByGame, RemoveGameFromCategory } from "../../wailsjs/go/service/CategoryService";
import { DeleteGame, GetGameByID, SelectCoverImage, SelectGameExecutable, SelectSaveDirectory, SelectSaveFile, UpdateGame, UpdateGameFromRemote } from "../../wailsjs/go/service/GameService";
import { StartGameWithTracking } from "../../wailsjs/go/service/StartService";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { GameBackupPanel } from "../components/panel/GameBackupPanel";
import { GameEditPanel } from "../components/panel/GameEditPanel";
import { GameLaunchPanel } from "../components/panel/GameLaunchPanel";
import { GameStatsPanel } from "../components/panel/GameStatsPanel";
import { GameDetailSkeleton } from "../components/skeleton/GameDetailSkeleton";
import { useAppStore } from "../store";
import { formatLocalDate } from "../utils/time";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/game/$gameId",
  component: GameDetailPage,
});

function GameDetailPage() {
  const navigate = useNavigate();
  const { gameId } = Route.useParams();
  const config = useAppStore(state => state.config);
  const { t } = useTranslation();
  const [game, setGame] = useState<models.Game | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [activeTab, setActiveTab] = useState("stats");
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);
  const [allCategories, setAllCategories] = useState<vo.CategoryVO[]>([]);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);
  const isInitialMount = useRef(true);
  const originalGameData = useRef<models.Game | null>(null);

  useEffect(() => {
    const loadData = async () => {
      try {
        const gameData = await GetGameByID(gameId);
        setGame(gameData);
        originalGameData.current = gameData;
        isInitialMount.current = false;
      }
      catch (error) {
        console.error("Failed to load game data:", error);
        toast.error(t("game.toast.loadDataFailed"));
      }
      finally {
        setIsLoading(false);
      }
    };
    loadData();
  }, [gameId, t]);

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number;
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [isLoading]);

  // 自动保存
  useEffect(() => {
    if (!game || isInitialMount.current)
      return;

    const hasChanges = JSON.stringify(game) !== JSON.stringify(originalGameData.current);
    if (!hasChanges)
      return;

    const timer = setTimeout(async () => {
      try {
        await UpdateGame(game);
        originalGameData.current = game;
      }
      catch (error) {
        console.error("Failed to auto-save game:", error);
        toast.error(t("game.toast.saveFailed", { error: (error as Error).message }));
      }
    }, 500);

    return () => clearTimeout(timer);
  }, [game, t]);

  if (isLoading && !game) {
    if (!showSkeleton) {
      return null;
    }
    return <GameDetailSkeleton />;
  }

  if (!game) {
    return (
      <div className="flex flex-col items-center justify-center h-full space-y-4 text-brand-500">
        <div className="i-mdi-gamepad-variant-outline text-6xl" />
        <p className="text-xl">{t("game.notFound")}</p>
        <button type="button" onClick={() => navigate({ to: "/library" })} className="text-neutral-600 hover:underline">{t("game.backToLibrary")}</button>
      </div>
    );
  }

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path && game) {
        setGame({ ...game, path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select executable:", error);
      toast.error(t("game.toast.selectExecutableFailed"));
    }
  };

  const handleDeleteGame = async () => {
    if (!game)
      return;
    setIsDeleteModalOpen(true);
  };

  const confirmDeleteGame = async () => {
    if (!game)
      return;
    try {
      await DeleteGame(game.id);
      toast.success(t("game.toast.deleteSuccess"));
      navigate({ to: "/library" });
    }
    catch (error) {
      console.error("Failed to delete game:", error);
      toast.error(t("game.toast.deleteFailed"));
    }
  };

  const handleSelectSaveDirectory = async () => {
    try {
      const path = await SelectSaveDirectory();
      if (path && game) {
        setGame({ ...game, save_path: path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select save directory:", error);
      toast.error(t("game.toast.selectSaveDirFailed"));
    }
  };

  const handleSelectSaveFile = async () => {
    try {
      const path = await SelectSaveFile();
      if (path && game) {
        setGame({ ...game, save_path: path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select save file:", error);
      toast.error(t("game.toast.selectSaveFileFailed"));
    }
  };

  const handleSelectCoverImage = async () => {
    if (!game)
      return;
    try {
      const coverUrl = await SelectCoverImage(game.id);
      if (coverUrl) {
        setGame({ ...game, cover_url: coverUrl } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select cover image:", error);
      toast.error(t("game.toast.selectCoverFailed"));
    }
  };

  const handleUpdateFromRemote = async () => {
    if (!game)
      return;
    try {
      await UpdateGameFromRemote(game.id);
      const updatedGame = await GetGameByID(game.id);
      setGame(updatedGame);
      originalGameData.current = updatedGame;
      toast.success(t("game.toast.updateRemoteSuccess"));
    }
    catch (error) {
      console.error("Failed to update from remote:", error);
      toast.error(t("game.toast.updateRemoteFailed", { error }));
    }
  };

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: { label: t("common.notStarted"), icon: "i-mdi-clock-outline", color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300" },
    [enums.GameStatus.PLAYING]: { label: t("common.playing"), icon: "i-mdi-gamepad-variant", color: "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300" },
    [enums.GameStatus.COMPLETED]: { label: t("common.completed"), icon: "i-mdi-trophy", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300" },
    [enums.GameStatus.ON_HOLD]: { label: t("common.onHold"), icon: "i-mdi-pause-circle-outline", color: "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300" },
  };

  const handleStartGame = async () => {
    if (!game || !game.id)
      return;
    try {
      const started = await StartGameWithTracking(game.id);
      if (started) {
        toast.success(t("gameCard.startSuccess", { name: game.name }));
      }
      else {
        toast.error(t("gameCard.startFailedNotLaunched", { name: game.name }));
      }
    }
    catch (error) {
      console.error("Failed to start game:", error);
      toast.error(t("gameCard.startFailedLog", { name: game.name }));
    }
  };

  const handleStatusChange = async (newStatus: string) => {
    if (!game)
      return;
    const updatedGame = { ...game, status: newStatus } as models.Game;
    setGame(updatedGame);
    try {
      await UpdateGame(updatedGame);
      toast.success(t("game.toast.statusUpdated"));
    }
    catch (error) {
      console.error("Failed to update status:", error);
      toast.error(t("game.toast.statusUpdateFailed"));
    }
  };

  const openCategoryModal = async () => {
    try {
      const [categories, gameCategories] = await Promise.all([
        GetCategories(),
        GetCategoriesByGame(gameId),
      ]);
      setAllCategories(categories || []);
      setSelectedCategoryIds(gameCategories?.map(c => c.id) || []);
      setIsCategoryModalOpen(true);
    }
    catch (error) {
      console.error("Failed to load categories:", error);
      toast.error(t("game.toast.loadFavFailed"));
    }
  };

  const handleSaveCategories = async (newSelectedIds: string[]) => {
    const currentIds = selectedCategoryIds;

    // 计算需要添加的和移除的
    const toAdd = newSelectedIds.filter(id => !currentIds.includes(id));
    const toRemove = currentIds.filter(id => !newSelectedIds.includes(id));

    try {
      // 执行添加操作
      for (const categoryId of toAdd) {
        await AddGameToCategory(gameId, categoryId);
      }
      // 执行移除操作
      for (const categoryId of toRemove) {
        await RemoveGameFromCategory(gameId, categoryId);
      }

      setSelectedCategoryIds(newSelectedIds);

      // 刷新所有分类的game_count
      const categories = await GetCategories();
      setAllCategories(categories || []);

      if (toAdd.length > 0 || toRemove.length > 0) {
        toast.success(t("game.toast.favUpdated"));
      }
    }
    catch (error) {
      console.error("Failed to update categories:", error);
      toast.error(t("game.toast.updateFavFailed"));
    }
  };

  const handleSelectProcessExecutable = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path && game) {
        // 从路径中提取文件名
        const filename = path.split(/[\\/]/).pop();
        if (filename) {
          setGame({ ...game, process_name: filename } as models.Game);
        }
      }
    }
    catch (error) {
      console.error("Failed to select executable:", error);
      toast.error(t("game.toast.selectFileFailed"));
    }
  };

  return (
    <div className={`space-y-8 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      {/* Back Button */}
      <button
        type="button"
        onClick={() => window.history.back()}
        className="flex rounded-md items-center text-brand-750 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-200 transition-colors"
      >
        <div className="i-mdi-arrow-left text-2xl mr-1" />
        <span>{t("common.back")}</span>
      </button>

      {/* Header Section */}
      <div className="flex gap-6 items-center">
        <div className="relative w-60 flex-shrink-0 rounded-lg overflow-hidden shadow-lg bg-brand-200 dark:bg-brand-800">
          {game.cover_url
            ? (
                <img
                  src={game.cover_url}
                  alt={game.name}
                  className="w-full h-auto block"
                  referrerPolicy="no-referrer"
                  draggable="false"
                  onDragStart={e => e.preventDefault()}
                />
              )
            : (
                <div className="w-full h-64 flex items-center justify-center text-brand-400">
                  {t("game.noCover")}
                </div>
              )}
        </div>

        <div className="flex-1 space-y-4">
          <div className="flex flex-col gap-3">
            <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{game.name}</h1>
            {/* 操作和状态标签组 */}
            <div className="flex items-center gap-4">
              <button
                type="button"
                onClick={handleStartGame}
                className="flex items-center gap-1.5 rounded-lg bg-neutral-600 text-white shadow-md hover:bg-neutral-700 transition-all duration-300 px-4 py-1.5 text-sm font-medium dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-200"
              >
                <div className="i-mdi-play text-lg" />
                {t("gameCard.startGame")}
              </button>

              <div className="h-6 w-px bg-brand-200 dark:bg-brand-700" />
              {" "}
              {/* 分隔线 */}

              <div className="flex gap-1.5">
                {Object.entries(statusConfig).map(([key, config]) => {
                  const isActive = (game.status || enums.GameStatus.NOT_STARTED) === key;
                  return (
                    <button
                      type="button"
                      key={key}
                      onClick={() => handleStatusChange(key)}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition-all ${isActive
                        ? `${config.color} ring-2 ring-offset-1 ring-brand-400 dark:ring-offset-brand-900`
                        : "bg-brand-100 text-brand-500 dark:bg-brand-700 dark:text-brand-400 hover:bg-brand-200 dark:hover:bg-brand-600"
                      }`}
                      title={config.label}
                    >
                      <div className={`${config.icon} text-base`} />
                      {isActive && <span>{config.label}</span>}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-4 gap-4 text-sm text-brand-750 dark:text-brand-400">
            <div>
              <div className="font-semibold mb-1">{t("game.dataSource")}</div>
              <div>{game.source_type}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">{t("game.developer")}</div>
              <div>{game.company || "-"}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">{t("common.createdAt")}</div>
              <div>{formatLocalDate(game.created_at, config?.time_zone)}</div>
            </div>
            {/* Placeholders for missing data */}
          </div>

          <div className="mt-4">
            <div className="font-semibold mb-2 text-brand-900 dark:text-white">{t("game.summary")}</div>
            <p className="text-brand-750 dark:text-brand-400 text-sm leading-relaxed whitespace-pre-wrap max-h-60 overflow-y-auto scrollbar-hide pr-2">
              {game.summary || t("game.noSummary")}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-brand-200 dark:border-brand-700">
        <div className="flex justify-between items-center">
          <nav className="-mb-px flex space-x-8">
            {["stats", "edit", "launch", "backup"].map(tab => (
              <button
                type="button"
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`
                  whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                  ${activeTab === tab
                ? "border-neutral-500 text-brand-700 dark:text-neutral-400"
                : "border-transparent text-brand-700 hover:text-brand-750 hover:border-brand-300 dark:text-brand-400 dark:hover:text-brand-300"}
                `}
              >
                {tab === "stats" && t("game.tabs.stats")}
                {tab === "edit" && t("common.edit")}
                {tab === "launch" && t("game.tabs.launch")}
                {tab === "backup" && t("game.tabs.backup")}
              </button>
            ))}
          </nav>
          <button
            type="button"
            onClick={openCategoryModal}
            className="flex items-center gap-2 px-3 py-2 rounded-lg bg-brand-100 text-brand-750 hover:text-brand-200 dark:bg-brand-900 dark:text-brand-400 dark:hover:text-brand-700 transition-colors"
            title={t("game.addToFav")}
          >
            <div className="i-mdi-folder-plus-outline text-lg" />
          </button>
        </div>
      </div>

      {/* Content */}
      {activeTab === "stats" && (
        <GameStatsPanel gameId={gameId} />
      )}

      {activeTab === "edit" && game && (
        <GameEditPanel
          game={game}
          onGameChange={setGame}
          onDelete={handleDeleteGame}
          onSelectExecutable={handleSelectExecutable}
          onSelectSaveDirectory={handleSelectSaveDirectory}
          onSelectSaveFile={handleSelectSaveFile}
          onSelectCoverImage={handleSelectCoverImage}
          onUpdateFromRemote={handleUpdateFromRemote}
        />
      )}

      {activeTab === "launch" && game && (
        <GameLaunchPanel
          game={game}
          config={config || undefined}
          onGameChange={setGame}
          onSelectProcessExecutable={handleSelectProcessExecutable}
        />
      )}

      {activeTab === "backup" && (
        <GameBackupPanel gameId={gameId} savePath={game?.save_path} />
      )}

      <ConfirmModal
        isOpen={isDeleteModalOpen}
        title={t("game.deleteGame")}
        message={t("game.deleteConfirmMsg", { name: game.name })}
        confirmText={t("game.confirmDelete")}
        type="danger"
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={confirmDeleteGame}
      />

      <AddToCategoryModal
        isOpen={isCategoryModalOpen}
        allCategories={allCategories}
        initialSelectedIds={selectedCategoryIds}
        onClose={() => setIsCategoryModalOpen(false)}
        onSave={handleSaveCategories}
      />
    </div>
  );
}
