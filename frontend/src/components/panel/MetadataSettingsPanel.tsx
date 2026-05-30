import type { appconf, vo } from "../../../wailsjs/go/models";
import type { MetadataRefreshProgress } from "../modal/MetadataRefreshProgressModal";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { RefreshAllGamesMetadata } from "../../../wailsjs/go/service/GameService";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { ConfirmModal } from "../modal/ConfirmModal";
import { MetadataRefreshProgressModal } from "../modal/MetadataRefreshProgressModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface MetadataSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

const DEFAULT_METADATA_SOURCES = ["bangumi", "vndb", "ymgal", "steam"];
const VALID_METADATA_SOURCES = [
  ...DEFAULT_METADATA_SOURCES,
  "dlsite",
  "erogamescape",
];
const DEFAULT_SCRAPED_TAG_LIMIT = 10;

function normalizeMetadataSources(sources?: string[]): string[] {
  const validSourceSet = new Set(VALID_METADATA_SOURCES);
  const normalized: string[] = [];

  for (const source of sources || []) {
    const lower = source?.toLowerCase().trim();
    if (!lower || !validSourceSet.has(lower) || normalized.includes(lower)) {
      continue;
    }
    normalized.push(lower);
  }

  return normalized.length > 0 ? normalized : [...DEFAULT_METADATA_SOURCES];
}

export function MetadataSettingsPanel({
  formData,
  onChange,
}: MetadataSettingsPanelProps) {
  const { t } = useTranslation();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshProgress, setRefreshProgress]
    = useState<MetadataRefreshProgress>({
      status: "idle",
      current: 0,
      total: 0,
      game_name: "",
      updated_games: 0,
      skipped_games: 0,
      failed_games: 0,
      locked_games: 0,
    });
  const [confirmConfig, setConfirmConfig] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    type: "danger" | "info";
    onConfirm: () => void;
  }>({
    isOpen: false,
    title: "",
    message: "",
    type: "info",
    onConfirm: () => {},
  });

  const selectedSources = normalizeMetadataSources(formData.metadata_sources);
  const scrapedTagLimit
    = typeof formData.scraped_tag_limit === "number"
      ? Math.max(-1, formData.scraped_tag_limit)
      : DEFAULT_SCRAPED_TAG_LIMIT;
  const isTagLimitUnlimited = scrapedTagLimit < 0;

  useEffect(() => {
    const unsubscribe = EventsOn(
      "metadata:refresh-progress",
      (evt: MetadataRefreshProgress) => {
        setRefreshProgress(evt);
      },
    );

    return unsubscribe;
  }, []);

  const sourceItems: Array<{
    value: string;
    label: string;
    hint: string;
    icon: string;
  }> = [
    {
      value: "bangumi",
      label: "Bangumi",
      hint: t("settings.metadata.sourceHints.bangumi"),
      icon: "/bangumi-logo.png",
    },
    {
      value: "vndb",
      label: "VNDB",
      hint: t("settings.metadata.sourceHints.vndb"),
      icon: "/vndb-logo.svg",
    },
    {
      value: "ymgal",
      label: "Ymgal",
      hint: t("settings.metadata.sourceHints.ymgal"),
      icon: "/ymgal-logo.png",
    },
    {
      value: "steam",
      label: "Steam",
      hint: t("settings.metadata.sourceHints.steam"),
      icon: "/steam-logo.png",
    },
    {
      value: "dlsite",
      label: "DLsite",
      hint: t("settings.metadata.sourceHints.dlsite"),
      icon: "/dlsite-logo.png",
    },
    {
      value: "erogamescape",
      label: "ErogameScape",
      hint: t("settings.metadata.sourceHints.erogamescape"),
      icon: "/erogamescape-logo.png",
    },
  ];

  const handleToggleSource = (source: string, checked: boolean) => {
    let nextSources = selectedSources;

    if (checked) {
      if (!selectedSources.includes(source)) {
        nextSources = [...selectedSources, source];
      }
    }
    else {
      nextSources = selectedSources.filter(item => item !== source);
      if (nextSources.length === 0) {
        toast.error(t("settings.metadata.toast.atLeastOneSource"));
        return;
      }
    }

    onChange({
      ...formData,
      metadata_sources: nextSources,
    } as appconf.AppConfig);
  };

  const handleRefreshAllMetadata = () => {
    if (isRefreshing) {
      return;
    }

    setConfirmConfig({
      isOpen: true,
      title: t("settings.metadata.modal.refreshTitle"),
      message: t("settings.metadata.modal.refreshMessage"),
      type: "danger",
      onConfirm: async () => {
        setIsRefreshing(true);
        setRefreshProgress({
          status: "started",
          current: 0,
          total: 0,
          game_name: "",
          updated_games: 0,
          skipped_games: 0,
          failed_games: 0,
          locked_games: 0,
        });
        try {
          const refreshResult: vo.MetadataRefreshResult
            = await RefreshAllGamesMetadata();
          toast.success(
            t("settings.metadata.toast.refreshSuccess", {
              updated: refreshResult.updated_games,
              failed: refreshResult.failed_games,
              skipped: refreshResult.skipped_games,
              locked: refreshResult.locked_games,
            }),
          );
        }
        catch (err) {
          toast.error(
            t("settings.metadata.toast.refreshFailed", { error: err }),
          );
        }
        finally {
          setIsRefreshing(false);
        }
      },
    });
  };

  const handleTagLimitChange = (value: string) => {
    const parsed = Number.parseInt(value, 10);
    onChange({
      ...formData,
      scraped_tag_limit: Number.isFinite(parsed) ? Math.max(0, parsed) : 0,
    } as appconf.AppConfig);
  };

  return (
    <>
      <div className="space-y-4">
        <div>
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.metadata.sourceTitle")}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {sourceItems.map(item => (
            <div
              key={item.value}
              className="glass-panel flex items-center justify-between rounded-lg border border-brand-200 p-4 dark:border-brand-700"
            >
              <div className="flex-1 space-y-2">
                <div className="flex items-center gap-2 select-none">
                  {item.icon && (
                    <img
                      src={item.icon}
                      alt={item.label}
                      className="h-[22px] w-auto object-contain brightness-0 opacity-80 transition-all dark:invert dark:opacity-90"
                    />
                  )}
                  <label
                    htmlFor={`metadata-source-${item.value}`}
                    className="block text-sm font-medium text-brand-700 dark:text-brand-300"
                  >
                    {item.label}
                  </label>
                </div>
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {item.hint}
                </p>
              </div>
              <BetterSwitch
                id={`metadata-source-${item.value}`}
                checked={selectedSources.includes(item.value)}
                onCheckedChange={checked =>
                  handleToggleSource(item.value, checked)}
              />
            </div>
          ))}
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="allow-duplicate-metadata-import"
                className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.metadata.allowDuplicateMetadataImport")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.metadata.allowDuplicateMetadataImportHint")}
              </p>
            </div>
            <BetterSwitch
              id="allow-duplicate-metadata-import"
              checked={Boolean(formData.allow_duplicate_metadata_import)}
              onCheckedChange={checked =>
                onChange({
                  ...formData,
                  allow_duplicate_metadata_import: checked,
                } as appconf.AppConfig)}
            />
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="enable-tag-translation"
                className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.metadata.enableTagTranslation")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.metadata.enableTagTranslationHint")}
              </p>
            </div>
            <BetterSwitch
              id="enable-tag-translation"
              checked={formData.enable_tag_translation !== false}
              onCheckedChange={checked =>
                onChange({
                  ...formData,
                  enable_tag_translation: checked,
                } as appconf.AppConfig)}
            />
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="scraped-tag-limit"
                className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.metadata.scrapedTagLimit")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.metadata.scrapedTagLimitHint")}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap items-center justify-end gap-3">
              <input
                id="scraped-tag-limit"
                type="number"
                min={0}
                step={1}
                disabled={isTagLimitUnlimited}
                value={isTagLimitUnlimited ? "" : scrapedTagLimit}
                onChange={event => handleTagLimitChange(event.target.value)}
                className="glass-input h-10 w-24 rounded-lg border border-brand-200 bg-white px-3 text-sm text-brand-900 outline-none transition-colors focus:ring-2 focus:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-brand-700 dark:bg-brand-900 dark:text-brand-100"
              />
              <label
                htmlFor="scraped-tag-limit-unlimited"
                className="cursor-pointer select-none text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.metadata.scrapedTagLimitUnlimited")}
              </label>
              <BetterSwitch
                id="scraped-tag-limit-unlimited"
                checked={isTagLimitUnlimited}
                onCheckedChange={checked =>
                  onChange({
                    ...formData,
                    scraped_tag_limit: checked ? -1 : DEFAULT_SCRAPED_TAG_LIMIT,
                  } as appconf.AppConfig)}
              />
            </div>
          </div>
        </div>
      </div>

      <div className="mt-6 border-brand-200 pt-6 dark:border-brand-700">
        <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.metadata.refreshTitle")}
        </div>
        <BetterButton
          className="mt-4 w-full justify-center sm:w-auto"
          variant="primary"
          icon="i-mdi-database-refresh"
          isLoading={isRefreshing}
          onClick={handleRefreshAllMetadata}
        >
          {isRefreshing
            ? t("settings.metadata.refreshing")
            : t("settings.metadata.refreshButton")}
        </BetterButton>
      </div>

      <ConfirmModal
        isOpen={confirmConfig.isOpen}
        title={confirmConfig.title}
        message={confirmConfig.message}
        type={confirmConfig.type}
        onClose={() => setConfirmConfig({ ...confirmConfig, isOpen: false })}
        onConfirm={confirmConfig.onConfirm}
      />
      <MetadataRefreshProgressModal
        isOpen={isRefreshing}
        progress={refreshProgress}
      />
    </>
  );
}
