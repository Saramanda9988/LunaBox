import type { appconf, vo } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { RefreshAllGamesMetadata } from "../../../wailsjs/go/service/GameService";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface MetadataSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

const DEFAULT_METADATA_SOURCES = ["bangumi", "vndb", "ymgal", "steam"];

function normalizeMetadataSources(sources?: string[]): string[] {
  const validSourceSet = new Set(DEFAULT_METADATA_SOURCES);
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
        try {
          const refreshResult: vo.MetadataRefreshResult
            = await RefreshAllGamesMetadata();
          toast.success(
            t("settings.metadata.toast.refreshSuccess", {
              updated: refreshResult.updated_games,
              failed: refreshResult.failed_games,
              skipped: refreshResult.skipped_games,
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

  return (
    <>
      <div className="space-y-4">
        <div>
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.metadata.sourceTitle")}
          </div>
        </div>

        <div className="space-y-3">
          {sourceItems.map(item => (
            <div
              key={item.value}
              className="glass-panel flex items-center justify-between rounded-lg border border-brand-200 p-4 dark:border-brand-700"
            >
              <div className="flex-1 space-y-2">
                <div className="flex items-center gap-2 select-none">
                  <img
                    src={item.icon}
                    alt={item.label}
                    className="h-[22px] w-auto object-contain brightness-0 opacity-80 transition-all dark:invert dark:opacity-90"
                  />
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
    </>
  );
}
