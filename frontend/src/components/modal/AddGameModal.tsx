import { useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums, models, vo } from "../../../wailsjs/go/models";
import {
  AddGameFromWebMetadata,
  FetchMetadataByName,
  FetchMetadataFromWeb,
  SelectCoverImageWithTempID,
  SelectGameExecutable,
} from "../../../wailsjs/go/service/GameService";
import { BetterSelect } from "../ui/better/BetterSelect";
import { ModalPortal } from "../ui/ModalPortal";

interface AddGameModalProps {
  isOpen: boolean;
  onClose: () => void;
  onGameAdded: () => void;
}

type StepType = "type" | "local" | "results" | "id" | "remote" | "manual";
type ImportMode = "local" | "remote";

export function AddGameModal({
  isOpen,
  onClose,
  onGameAdded,
}: AddGameModalProps) {
  const [step, setStep] = useState<StepType>("type");
  const [importMode, setImportMode] = useState<ImportMode>("local");
  const [executablePath, setExecutablePath] = useState("");
  const [gameName, setGameName] = useState("");
  const [metadataResults, setMetadataResults] = useState<
    vo.GameMetadataFromWebVO[]
  >([]);
  const [isLoading, setIsLoading] = useState(false);
  const { t } = useTranslation();
  const [manualId, setManualId] = useState("");
  const [manualSource, setManualSource] = useState<enums.SourceType>(
    enums.SourceType.BANGUMI,
  );

  const [manualCoverUrl, setManualCoverUrl] = useState("");
  const [manualCompany, setManualCompany] = useState("");
  const [manualSummary, setManualSummary] = useState("");

  if (!isOpen)
    return null;

  const sourceOptions = [
    { value: enums.SourceType.BANGUMI, label: "Bangumi" },
    { value: enums.SourceType.VNDB, label: "VNDB" },
    {
      value: enums.SourceType.YMGAL,
      label: t("gameEdit.sourceYmgal"),
    },
    {
      value: enums.SourceType.DLSITE,
      label: t("gameEdit.sourceDlsite"),
    },
    {
      value: enums.SourceType.EROGAMESCAPE,
      label: t("gameEdit.sourceErogameScape"),
    },
    { value: enums.SourceType.STEAM, label: "Steam" },
  ];

  const isRemoteImport = importMode === "remote";
  const entryStep: StepType = isRemoteImport ? "remote" : "local";

  const resetAndClose = () => {
    setStep("type");
    setImportMode("local");
    setExecutablePath("");
    setGameName("");
    setMetadataResults([]);
    setManualId("");
    setManualCoverUrl("");
    setManualCompany("");
    setManualSummary("");
    onClose();
  };

  const startLocalImport = () => {
    setImportMode("local");
    setManualId("");
    setMetadataResults([]);
    setStep("local");
  };

  const startRemoteImport = () => {
    setImportMode("remote");
    setExecutablePath("");
    setManualId("");
    setMetadataResults([]);
    setStep("remote");
  };

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path) {
        setExecutablePath(path);
        const normalizedPath = path.replace(/\\/g, "/");
        const parts = normalizedPath.split("/");
        if (parts.length > 1) {
          setGameName(parts[parts.length - 2]);
        }
      }
    }
    catch (error) {
      console.error("Failed to select executable:", error);
      toast.error(t("addGameModal.toast.openSelectorFailed"));
    }
  };

  const handleSearchByName = async () => {
    if (!gameName)
      return;
    setIsLoading(true);
    try {
      const results = await FetchMetadataByName(gameName);
      setMetadataResults(results || []);
      setStep("results");
    }
    catch (error) {
      console.error("Failed to fetch metadata:", error);
      toast.error(t("addGameModal.toast.fetchMetaFailed"));
    }
    finally {
      setIsLoading(false);
    }
  };

  const applyImportFields = (game: models.Game) => {
    game.path = isRemoteImport ? "" : executablePath;
    game.status = isRemoteImport
      ? enums.GameStatus.WANT_TO_PLAY
      : game.status || enums.GameStatus.NOT_STARTED;
  };

  const saveGameFromWebMetadata = async (meta: vo.GameMetadataFromWebVO) => {
    try {
      const gameMeta = vo.GameMetadataFromWebVO.createFrom(meta);
      if (!gameMeta.Game) {
        toast.error(t("addGameModal.toast.saveGameFailed"));
        return;
      }
      applyImportFields(gameMeta.Game);
      await AddGameFromWebMetadata(gameMeta);
      onGameAdded();
      resetAndClose();
    }
    catch (error) {
      console.error("Failed to save game from metadata:", error);
      toast.error(t("addGameModal.toast.saveGameFailed"));
    }
  };

  const handleSearchById = async () => {
    if (!manualId)
      return;
    setIsLoading(true);
    try {
      const request = new vo.MetadataRequest({
        source: manualSource,
        id: manualId,
      });
      const metadata = await FetchMetadataFromWeb(request);
      if (metadata && metadata.Game) {
        await saveGameFromWebMetadata(metadata);
      }
    }
    catch (error) {
      console.error("Failed to fetch metadata by ID:", error);
      toast.error(t("addGameModal.toast.fetchMetaByIdFailed"));
    }
    finally {
      setIsLoading(false);
    }
  };

  const handleSelectCoverImage = async () => {
    try {
      const coverUrl = await SelectCoverImageWithTempID();
      if (coverUrl) {
        setManualCoverUrl(coverUrl);
      }
    }
    catch (error) {
      console.error("Failed to select cover image:", error);
      toast.error(t("addGameModal.toast.selectCoverFailed"));
    }
  };

  const handleManualSave = async () => {
    if (!gameName) {
      toast.error(t("addGameModal.toast.fillGameName"));
      return;
    }
    setIsLoading(true);
    try {
      const game = new models.Game({
        name: gameName,
        path: isRemoteImport ? "" : executablePath,
        cover_url: manualCoverUrl,
        company: manualCompany,
        summary: manualSummary,
        source_type: isRemoteImport ? manualSource : enums.SourceType.LOCAL,
        status: isRemoteImport
          ? enums.GameStatus.WANT_TO_PLAY
          : enums.GameStatus.NOT_STARTED,
      });
      await AddGameFromWebMetadata(
        new vo.GameMetadataFromWebVO({
          Source: isRemoteImport ? manualSource : enums.SourceType.LOCAL,
          Game: game,
          Tags: [],
        }),
      );
      onGameAdded();
      resetAndClose();
    }
    catch (error) {
      console.error("Failed to save game manually:", error);
      toast.error(t("addGameModal.toast.saveGameFailed"));
    }
    finally {
      setIsLoading(false);
    }
  };

  const renderSourceAndIdFields = () => (
    <>
      <div>
        <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
          {t("addGameModal.dataSource")}
        </label>
        <BetterSelect
          value={manualSource}
          onChange={value => setManualSource(value as enums.SourceType)}
          options={sourceOptions}
        />
      </div>

      <div>
        <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
          {t("addGameModal.gameId")}
        </label>
        <input
          type="text"
          value={manualId}
          onChange={e => setManualId(e.target.value)}
          placeholder={t("addGameModal.gameIdPlaceholder")}
          className="box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
        />
      </div>
    </>
  );

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="w-full max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-brand-800">
          <div className="mb-6 flex items-center justify-between">
            <h2 className="text-4xl font-bold text-brand-900 dark:text-white">
              {t("library.addGame")}
            </h2>
            <button
              onClick={resetAndClose}
              className="i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 focus:outline-none dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
              aria-label={t("common.cancel")}
            />
          </div>

          {step === "type" && (
            <div className="space-y-4">
              <p className="text-sm text-brand-600 dark:text-brand-300">
                {t("addGameModal.chooseImportType")}
              </p>

              <div className="grid gap-4 sm:grid-cols-2">
                <button
                  type="button"
                  onClick={startLocalImport}
                  className="group relative min-h-56 overflow-hidden rounded-xl border border-brand-200 p-5 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-md dark:border-brand-700"
                >
                  <img
                    src="/luna1.webp"
                    alt=""
                    aria-hidden="true"
                    className="absolute bottom-0 right-0 h-[92%] w-[78%] object-contain object-bottom opacity-65 "
                    draggable="false"
                  />
                  <span className="absolute inset-0 dark:bg-brand-950/20" />
                  <span className="absolute inset-0 bg-gradient-to-r from-white/80 via-white/42 to-transparent dark:from-brand-900/72 dark:via-brand-900/36 dark:to-transparent" />
                  <span className="relative flex min-h-46 flex-col justify-end">
                    <span className="block text-lg font-semibold text-brand-900 dark:text-white">
                      {t("addGameModal.localGame")}
                    </span>
                    <span className="mt-2 block text-xs leading-5 text-brand-700 dark:text-brand-200">
                      {t("addGameModal.localGameHint")}
                    </span>
                  </span>
                </button>

                <button
                  type="button"
                  onClick={startRemoteImport}
                  className="group relative min-h-56 overflow-hidden rounded-xl border border-brand-200 p-5 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-md dark:border-brand-700"
                >
                  <img
                    src="/luna2.webp"
                    alt=""
                    aria-hidden="true"
                    className="absolute bottom-0 right-0 h-[92%] w-[78%] object-contain object-bottom opacity-65"
                    draggable="false"
                  />
                  <span className="absolute inset-0 dark:bg-brand-950/20" />
                  <span className="absolute inset-0 bg-gradient-to-r from-white/80 via-white/42 to-transparent dark:from-brand-900/72 dark:via-brand-900/36 dark:to-transparent" />
                  <span className="relative flex min-h-46 flex-col justify-end">
                    <span className="block text-lg font-semibold text-brand-900 dark:text-white">
                      {t("addGameModal.remoteGame")}
                    </span>
                    <span className="mt-2 block text-xs leading-5 text-brand-700 dark:text-brand-200">
                      {t("addGameModal.remoteGameHint")}
                    </span>
                  </span>
                </button>
              </div>
            </div>
          )}

          {step === "local" && (
            <div className="space-y-6">
              <button
                onClick={handleSelectExecutable}
                className="flex w-full items-center justify-center rounded-lg bg-neutral-500 py-4 text-white transition hover:bg-neutral-600"
              >
                <div className="i-mdi-file-find mr-2 text-xl" />
                {t("addGameModal.selectExecutable")}
              </button>

              <div>
                <input
                  type="text"
                  value={executablePath}
                  readOnly
                  placeholder={t("addGameModal.executablePlaceholder")}
                  className="box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
              </div>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.gameName")}
                </label>
                <input
                  type="text"
                  value={gameName}
                  onChange={e => setGameName(e.target.value)}
                  className="box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
              </div>

              <div className="flex justify-between gap-4">
                <button
                  onClick={() => setStep("type")}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  {t("common.back")}
                </button>
                <div className="flex gap-4">
                  <button
                    onClick={() => setStep("manual")}
                    disabled={!executablePath || !gameName}
                    className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 disabled:opacity-50 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                  >
                    {t("common.manualAdd")}
                  </button>
                  <button
                    onClick={handleSearchByName}
                    disabled={!executablePath || !gameName || isLoading}
                    className="rounded-lg bg-neutral-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
                  >
                    {isLoading
                      ? t("common.searching")
                      : t("addGameModal.searchMeta")}
                  </button>
                </div>
              </div>
            </div>
          )}

          {step === "remote" && (
            <div className="space-y-6">
              <div className="grid gap-5 sm:grid-cols-2">
                <div>
                  <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                    {t("addGameModal.dataSource")}
                  </label>
                  <BetterSelect
                    value={manualSource}
                    onChange={value =>
                      setManualSource(value as enums.SourceType)}
                    options={sourceOptions}
                    className="[&>button]:h-12 [&>button]:box-border [&>button]:items-center [&>button]:rounded-lg [&>button]:py-0"
                  />
                </div>

                <div>
                  <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                    {t("addGameModal.gameId")}
                  </label>
                  <div className="flex gap-3">
                    <input
                      type="text"
                      value={manualId}
                      onChange={e => setManualId(e.target.value)}
                      placeholder={t("addGameModal.gameIdPlaceholder")}
                      className="box-border block h-12 min-w-0 flex-1 rounded-lg border border-brand-300 bg-brand-50 px-3 py-0 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                    />
                    <button
                      onClick={handleSearchById}
                      disabled={!manualId || isLoading}
                      className="h-12 shrink-0 rounded-lg bg-neutral-600 px-5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
                    >
                      {isLoading ? t("common.searching") : t("common.confirm")}
                    </button>
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <div className="h-px flex-1 bg-brand-200 dark:bg-brand-700" />
                <span className="text-xs text-brand-500 dark:text-brand-400">
                  {t("addGameModal.orSearchByName")}
                </span>
                <div className="h-px flex-1 bg-brand-200 dark:bg-brand-700" />
              </div>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.gameName")}
                </label>
                <div className="flex gap-3">
                  <input
                    type="text"
                    value={gameName}
                    onChange={e => setGameName(e.target.value)}
                    className="box-border block flex-1 rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                  />
                  <button
                    onClick={handleSearchByName}
                    disabled={!gameName || isLoading}
                    className="rounded-lg bg-neutral-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
                  >
                    {isLoading
                      ? t("common.searching")
                      : t("addGameModal.searchMeta")}
                  </button>
                </div>
              </div>

              <div className="flex justify-between gap-4">
                <button
                  onClick={() => setStep("type")}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  {t("common.back")}
                </button>
                <button
                  onClick={() => setStep("manual")}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 disabled:opacity-50 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  {t("common.manualAdd")}
                </button>
              </div>
            </div>
          )}

          {step === "results" && (
            <div className="space-y-6">
              <p className="text-brand-600 dark:text-brand-300">
                {t("addGameModal.whichResult")}
              </p>

              <div className="flex w-full snap-x gap-4 overflow-x-auto p-2 pb-6 pt-2 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
                {metadataResults
                  .filter(item => item.Game)
                  .map((item, index) => (
                    <div
                      key={index}
                      onClick={() => saveGameFromWebMetadata(item)}
                      className="w-36 shrink-0 snap-center cursor-pointer rounded-xl border border-brand-200 bg-brand-50/50 p-3 shadow-sm transition-all hover:-translate-y-1 hover:border-brand-400 hover:shadow-md dark:border-brand-700 dark:bg-brand-800/50 dark:hover:border-brand-500 sm:w-40"
                    >
                      <div className="aspect-[3/4] w-full overflow-hidden rounded-md bg-brand-200 dark:bg-brand-700">
                        {item.Game!.cover_url ? (
                          <img
                            src={item.Game!.cover_url}
                            alt={item.Game!.name}
                            className="h-full w-full object-cover"
                            referrerPolicy="no-referrer"
                            draggable="false"
                            onDragStart={e => e.preventDefault()}
                          />
                        ) : (
                          <div className="flex h-full items-center justify-center text-brand-400">
                            <div className="i-mdi-image-off text-4xl" />
                          </div>
                        )}
                      </div>
                      <h3
                        className="mt-2 truncate text-sm font-bold text-brand-900 dark:text-white"
                        title={item.Game!.name}
                      >
                        {item.Game!.name}
                      </h3>
                      <p className="text-xs text-brand-500 dark:text-brand-400">
                        {t("addGameModal.fromSource", { source: item.Source })}
                      </p>
                    </div>
                  ))}
              </div>

              <div className="flex items-center justify-between border-t border-brand-200 pt-4 dark:border-brand-700">
                <button
                  onClick={() => setStep(entryStep)}
                  className="text-sm text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-200"
                >
                  &larr;
                  {t("addGameModal.goBack")}
                </button>
                <div className="flex space-x-4">
                  <div className="text-sm text-brand-500 dark:text-brand-400">
                    {t("addGameModal.noneOfAbove")}
                  </div>
                  <button
                    onClick={() => setStep("manual")}
                    className="text-sm text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-200"
                  >
                    {t("addGameModal.fillManually")}
                  </button>
                  <button
                    onClick={() => setStep(isRemoteImport ? "remote" : "id")}
                    className="text-sm text-neutral-600 hover:text-neutral-800 dark:text-neutral-400 dark:hover:text-neutral-300"
                  >
                    {t("addGameModal.searchById")}
                  </button>
                </div>
              </div>
            </div>
          )}

          {step === "id" && (
            <div className="space-y-6">
              {renderSourceAndIdFields()}

              <div className="flex justify-end space-x-4">
                <button
                  onClick={() => setStep("results")}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  {t("common.back")}
                </button>
                <button
                  onClick={handleSearchById}
                  disabled={!manualId || isLoading}
                  className="rounded-lg bg-neutral-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
                >
                  {isLoading ? t("common.searching") : t("common.confirm")}
                </button>
              </div>
            </div>
          )}

          {step === "manual" && (
            <div className="space-y-4">
              <p className="text-brand-600 dark:text-brand-300">
                {isRemoteImport
                  ? t("addGameModal.remoteManualFillInfo")
                  : t("addGameModal.manualFillInfo")}
              </p>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.gameName")}
                </label>
                <input
                  type="text"
                  value={gameName}
                  onChange={e => setGameName(e.target.value)}
                  className="box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
              </div>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.coverImage")}
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={manualCoverUrl}
                    onChange={e => setManualCoverUrl(e.target.value)}
                    placeholder={t("addGameModal.coverPlaceholder")}
                    className="box-border block flex-1 rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                  />
                  <button
                    type="button"
                    onClick={handleSelectCoverImage}
                    className="rounded-lg bg-brand-100 px-4 py-2 text-brand-700 hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
                  >
                    {t("common.select")}
                  </button>
                </div>
                <p className="mt-1 text-xs text-brand-500">
                  {t("addGameModal.coverHint")}
                </p>
              </div>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.developer")}
                </label>
                <input
                  type="text"
                  value={manualCompany}
                  onChange={e => setManualCompany(e.target.value)}
                  className="box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
              </div>

              <div>
                <label className="mb-2 block text-sm font-medium text-brand-900 dark:text-white">
                  {t("addGameModal.summary")}
                </label>
                <textarea
                  value={manualSummary}
                  onChange={e => setManualSummary(e.target.value)}
                  rows={3}
                  className="box-border block w-full resize-none rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
              </div>

              <div className="flex justify-end space-x-4 pt-2">
                <button
                  onClick={() => setStep(entryStep)}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  {t("common.back")}
                </button>
                <button
                  onClick={handleManualSave}
                  disabled={!gameName || isLoading}
                  className="rounded-lg bg-neutral-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50"
                >
                  {isLoading ? t("common.saving") : t("common.save")}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </ModalPortal>
  );
}
