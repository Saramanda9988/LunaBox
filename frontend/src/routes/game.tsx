import type { models, vo } from "../../wailsjs/go/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
import {
  AddGameToCategory,
  GetCategories,
  GetCategoriesByGame,
  RemoveGameFromCategory,
} from "../../wailsjs/go/service/CategoryService";
import {
  DeleteGame,
  ExportLaunchShortcut,
  GetGameByID,
  SelectCoverImage,
  SelectGameExecutable,
  SelectSaveDirectory,
  SelectSaveFile,
  UpdateGame,
  UpdateGameFromRemoteWithFields,
} from "../../wailsjs/go/service/GameService";
import {
  StartGameWithOptions,
  StartGameWithTracking,
} from "../../wailsjs/go/service/StartService";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import {
  DEFAULT_METADATA_UPDATE_FIELDS,
  MetadataFieldSelectModal,
} from "../components/modal/MetadataFieldSelectModal";
import { GameBackupPanel } from "../components/panel/GameBackupPanel";
import { GameEditPanel } from "../components/panel/GameEditPanel";
import { GameLaunchPanel } from "../components/panel/GameLaunchPanel";
import { GameProgressPanel } from "../components/panel/GameProgressPanel";
import { GameStatsPanel } from "../components/panel/GameStatsPanel";
import { GameDetailSkeleton } from "../components/skeleton/GameDetailSkeleton";
import { BetterSplitButton } from "../components/ui/better/BetterSplitButton";
import { GameTags } from "../components/ui/GameTags";
import { ProxyImage } from "../components/ui/ProxyImage";
import { useAppStore } from "../store";
import { formatLocalDate } from "../utils/time";
import { Route as rootRoute } from "./__root";

type LaunchMode = "normal" | "admin";

function isManagedLocalCoverURL(coverURL: string): boolean {
  return (
    coverURL.startsWith("/local/covers/")
    || /^https?:\/\/wails\.localhost(?::\d+)?\/local\/covers\//.test(coverURL)
  );
}

function buildCoverImageSrc(coverURL: string, refreshKey: string): string {
  if (!isManagedLocalCoverURL(coverURL)) {
    return coverURL;
  }

  const separator = coverURL.includes("?") ? "&" : "?";
  return `${coverURL}${separator}v=${encodeURIComponent(refreshKey)}`;
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/game/$gameId",
  component: GameDetailPage,
});

function GameDetailPage() {
  const navigate = useNavigate();
  const { gameId } = Route.useParams();
  const config = useAppStore(state => state.config);
  const platformGOOS = useAppStore(state => state.platformGOOS);
  const { t } = useTranslation();
  const [game, setGame] = useState<models.Game | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [activeTab, setActiveTab] = useState(() =>
    window.location.hash === "#launch" ? "launch" : "stats",
  );
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);
  const [isMetadataFieldModalOpen, setIsMetadataFieldModalOpen]
    = useState(false);
  const [isUpdatingFromRemote, setIsUpdatingFromRemote] = useState(false);
  const [selectedMetadataFields, setSelectedMetadataFields] = useState<
    enums.MetadataUpdateField[]
  >(DEFAULT_METADATA_UPDATE_FIELDS);
  const [allCategories, setAllCategories] = useState<vo.CategoryVO[]>([]);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);
  const [tagRefreshToken, setTagRefreshToken] = useState(0);
  const [launchMode, setLaunchMode] = useState<LaunchMode>("normal");
  const [coverImageRefreshToken, setCoverImageRefreshToken] = useState(() =>
    Date.now(),
  );
  const isInitialMount = useRef(true);
  const originalGameData = useRef<models.Game | null>(null);

  const navigateToLibrary = () => {
    navigate({ to: "/library" });
  };

  const searchLibraryByCompany = (company: string) => {
    navigate({
      to: "/library",
      search: { searchQuery: company },
    });
  };

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

  useEffect(() => {
    const syncTabFromHash = () => {
      if (window.location.hash === "#launch") {
        setActiveTab("launch");
      }
    };
    syncTabFromHash();
    window.addEventListener("hashchange", syncTabFromHash);
    return () => window.removeEventListener("hashchange", syncTabFromHash);
  }, []);

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

    const hasChanges
      = JSON.stringify(game) !== JSON.stringify(originalGameData.current);
    if (!hasChanges)
      return;

    const timer = setTimeout(async () => {
      try {
        await UpdateGame(game);
        originalGameData.current = game;
      }
      catch (error) {
        console.error("Failed to auto-save game:", error);
        toast.error(
          t("game.toast.saveFailed", { error: (error as Error).message }),
        );
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
        <button
          type="button"
          onClick={navigateToLibrary}
          className="text-neutral-600 hover:underline"
        >
          {t("game.backToLibrary")}
        </button>
      </div>
    );
  }

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable(game.path || "");
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
      navigateToLibrary();
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
        setCoverImageRefreshToken(prev => prev + 1);
      }
    }
    catch (error) {
      console.error("Failed to select cover image:", error);
      toast.error(t("game.toast.selectCoverFailed"));
    }
  };

  const handleOpenUpdateFromRemote = () => {
    if (!game || game.metadata_locked)
      return;
    setIsMetadataFieldModalOpen(true);
  };

  const handleUpdateFromRemote = async (
    fields: enums.MetadataUpdateField[],
  ) => {
    if (!game)
      return;
    const updateFields
      = fields.length > 0 ? fields : DEFAULT_METADATA_UPDATE_FIELDS;
    setSelectedMetadataFields(updateFields);
    setIsMetadataFieldModalOpen(false);
    setIsUpdatingFromRemote(true);
    try {
      await UpdateGameFromRemoteWithFields(game.id, updateFields);
      const updatedGame = await GetGameByID(game.id);
      setGame(updatedGame);
      originalGameData.current = updatedGame;
      setTagRefreshToken(prev => prev + 1);
      setCoverImageRefreshToken(prev => prev + 1);
      toast.success(t("game.toast.updateRemoteSuccess"));
    }
    catch (error) {
      console.error("Failed to update from remote:", error);
      toast.error(t("game.toast.updateRemoteFailed", { error }));
    }
    finally {
      setIsUpdatingFromRemote(false);
    }
  };

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: {
      label: t("common.notStarted"),
      icon: "i-mdi-clock-outline",
      color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300",
    },
    [enums.GameStatus.WANT_TO_PLAY]: {
      label: t("common.wantToPlay"),
      icon: "i-mdi-bookmark-outline",
      color: "bg-info-100 text-info-700 dark:bg-info-900 dark:text-info-300",
    },
    [enums.GameStatus.PLAYING]: {
      label: t("common.playing"),
      icon: "i-mdi-gamepad-variant",
      color:
        "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300",
    },
    [enums.GameStatus.COMPLETED]: {
      label: t("common.completed"),
      icon: "i-mdi-trophy",
      color:
        "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    },
    [enums.GameStatus.ON_HOLD]: {
      label: t("common.onHold"),
      icon: "i-mdi-pause-circle-outline",
      color:
        "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
    },
  };

  const handleStartGame = async (mode: LaunchMode = launchMode) => {
    if (!game || !game.id)
      return;
    try {
      const started
        = mode === "admin"
          ? await StartGameWithOptions(game.id, { RunAsAdmin: true })
          : await StartGameWithTracking(game.id);
      if (started) {
        try {
          const updatedGame = await GetGameByID(game.id);
          setGame(updatedGame);
          originalGameData.current = updatedGame;
        }
        catch (refreshError) {
          console.error("Failed to refresh game after start:", refreshError);
        }
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
    if (!game || (game.status || enums.GameStatus.NOT_STARTED) === newStatus)
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
      const path = await SelectGameExecutable(game.path || "");
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

  const handleExportLaunchShortcut = async () => {
    if (!game)
      return;
    try {
      const savePath = await ExportLaunchShortcut(game.id);
      if (!savePath) {
        return;
      }
      toast.success(
        t("gameLaunch.toast.shortcutExportSuccess", { path: savePath }),
      );
    }
    catch (error) {
      console.error("Failed to export launch shortcut:", error);
      toast.error(t("gameLaunch.toast.shortcutExportFailed", { error }));
    }
  };

  const ratingText = game.rating > 0 ? `${game.rating.toFixed(1)} / 10` : "-";
  const createdAtText = formatLocalDate(
    game.created_at,
    config?.time_zone,
  ).replaceAll("/", "-");
  const releaseDateText = game.release_date?.trim() || "-";
  const coverImageSrc = game.cover_url
    ? buildCoverImageSrc(game.cover_url, String(coverImageRefreshToken))
    : "";
  const launchOptions: Array<{
    key: LaunchMode;
    label: string;
    description: string;
    icon: string;
  }> = [
    {
      key: "normal",
      label: t("gameCard.startGame"),
      description: t("gameCard.normalLaunchDesc"),
      icon: "i-mdi-play",
    },
    {
      key: "admin",
      label: t("gameCard.startAsAdmin"),
      description: t("gameCard.adminLaunchDesc"),
      icon: "i-mdi-shield-account",
    },
  ];
  const selectedLaunchOption
    = launchOptions.find(option => option.key === launchMode)
      ?? launchOptions[0];

  return (
    <div
      className={`space-y-8 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}
    >
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
      <div className="grid min-w-0 grid-cols-[15rem_minmax(0,1fr)] items-center gap-6">
        <div className="relative w-60 rounded-lg overflow-hidden shadow-lg bg-brand-200 dark:bg-brand-800">
          {coverImageSrc ? (
            <ProxyImage
              src={coverImageSrc}
              alt={game.name}
              className="w-full h-auto block"
            />
          ) : (
            <div className="w-full h-64 flex items-center justify-center text-brand-400">
              {t("game.noCover")}
            </div>
          )}
        </div>

        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex flex-col gap-3">
            <h1 className="break-words text-4xl font-bold text-brand-900 dark:text-white">
              {game.name}
            </h1>
            {/* 操作和状态标签组 */}
            <div className="flex flex-wrap items-center gap-4">
              <BetterSplitButton
                label={selectedLaunchOption.label}
                icon={selectedLaunchOption.icon}
                selectedKey={launchMode}
                options={launchOptions}
                onClick={() => handleStartGame()}
                onSelect={setLaunchMode}
                size="sm"
                variant="primary"
                menuTitle={t("gameCard.launchMode")}
              />
              <div className="h-6 w-px bg-brand-200 dark:bg-brand-700" />
              {" "}
              {/* 分隔线 */}
              <div className="flex flex-wrap gap-1.5">
                {Object.entries(statusConfig).map(([key, config]) => {
                  const isActive
                    = (game.status || enums.GameStatus.NOT_STARTED) === key;
                  return (
                    <button
                      type="button"
                      key={key}
                      onClick={() => handleStatusChange(key)}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition-all ${
                        isActive
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

          <div className="grid min-w-0 grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4 text-sm text-brand-750 dark:text-brand-400">
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.dataSource")}</div>
              <div className="break-words">{game.source_type}</div>
            </div>
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.developer")}</div>
              {game.company?.trim() ? (
                <button
                  type="button"
                  onClick={() => searchLibraryByCompany(game.company.trim())}
                  className="max-w-full break-all text-left text-brand-750 dark:text-brand-400"
                >
                  {game.company}
                </button>
              ) : (
                <div>-</div>
              )}
            </div>
            <div>
              <div className="font-semibold mb-1">{t("common.createdAt")}</div>
              <div>{createdAtText}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">{t("game.rating")}</div>
              <div>{ratingText}</div>
            </div>
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.releaseDate")}</div>
              <div className="break-words">{releaseDateText}</div>
            </div>
          </div>

          <div className="mt-4 min-w-0">
            <div className="font-semibold mb-2 text-brand-900 dark:text-white">
              {t("game.summary")}
            </div>
            <p className="max-w-full break-words text-brand-750 dark:text-brand-400 text-sm leading-relaxed whitespace-pre-wrap max-h-60 overflow-y-auto overflow-x-hidden scrollbar-hide pr-2 [overflow-wrap:anywhere]">
              {game.summary || t("game.noSummary")}
            </p>
          </div>

          <div className="mt-3 min-w-0">
            <GameTags
              gameId={gameId}
              showNSFW={config?.show_nsfw_tags}
              refreshToken={tagRefreshToken}
            />
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-brand-200 dark:border-brand-700">
        <div className="flex justify-between items-center">
          <nav className="-mb-px flex space-x-8">
            {["stats", "edit", "launch", "backup", "progress"].map(tab => (
              <button
                type="button"
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`
                  whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                  ${
              activeTab === tab
                ? "border-neutral-500 text-brand-700 dark:text-neutral-400"
                : "border-transparent text-brand-700 hover:text-brand-750 hover:border-brand-300 dark:text-brand-400 dark:hover:text-brand-300"
              }
                `}
              >
                {tab === "stats" && t("game.tabs.stats")}
                {tab === "edit" && t("common.edit")}
                {tab === "launch" && t("game.tabs.launch")}
                {tab === "backup" && t("game.tabs.backup")}
                {tab === "progress" && t("game.tabs.progress")}
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
      {activeTab === "stats" && <GameStatsPanel gameId={gameId} />}

      {activeTab === "edit" && game && (
        <GameEditPanel
          game={game}
          onGameChange={setGame}
          onDelete={handleDeleteGame}
          onSelectExecutable={handleSelectExecutable}
          onSelectSaveDirectory={handleSelectSaveDirectory}
          onSelectSaveFile={handleSelectSaveFile}
          onSelectCoverImage={handleSelectCoverImage}
          onCoverImageChanged={() =>
            setCoverImageRefreshToken(prev => prev + 1)}
          onUpdateFromRemote={handleOpenUpdateFromRemote}
        />
      )}

      {activeTab === "launch" && game && (
        <GameLaunchPanel
          game={game}
          config={config || undefined}
          goos={platformGOOS}
          onGameChange={setGame}
          onSelectProcessExecutable={handleSelectProcessExecutable}
          onExportShortcut={handleExportLaunchShortcut}
        />
      )}

      {activeTab === "backup" && (
        <GameBackupPanel gameId={gameId} savePath={game?.save_path} />
      )}

      {activeTab === "progress" && <GameProgressPanel gameId={gameId} />}

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

      <MetadataFieldSelectModal
        isOpen={isMetadataFieldModalOpen}
        title={t("metadataUpdateFields.modal.singleTitle")}
        description={t("metadataUpdateFields.modal.singleDescription")}
        confirmText={t("metadataUpdateFields.modal.update")}
        initialFields={selectedMetadataFields}
        isSubmitting={isUpdatingFromRemote}
        onClose={() => setIsMetadataFieldModalOpen(false)}
        onConfirm={handleUpdateFromRemote}
      />
    </div>
  );
}
