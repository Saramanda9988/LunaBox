import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { enums } from "../../../src/bindings/models";
import { BetterDrawer } from "../ui/better/BetterDrawer";
import { BetterSwitch } from "../ui/better/BetterSwitch";
import { TOPBAR_HEIGHT } from "./TopBar";

interface SortOption {
  label: string;
  value: string;
}

interface FilterOption {
  label: string;
  value: enums.GameStatus | "";
}

interface FilterBarProps {
  searchQuery: string;
  onSearchChange: (value: string) => void;
  searchPlaceholder?: string;
  disableStoredSearchQuery?: boolean;
  sortBy: string;
  onSortByChange: (value: string) => void;
  sortOptions: SortOption[];
  sortOrder: enums.SortOrder;
  onSortOrderChange: (order: enums.SortOrder) => void;
  // 在卡片上展示当前排序字段对应的值（封面底部覆盖条）
  showSortField?: boolean;
  onShowSortFieldChange?: (value: boolean) => void;
  // 状态筛选
  statusFilter?: enums.GameStatus | "";
  onStatusFilterChange?: (value: enums.GameStatus | "") => void;
  statusFilterInverted?: boolean;
  onStatusFilterInvertedChange?: (value: boolean) => void;
  statusOptions?: FilterOption[];
  // 额外筛选内容（例如 tag 筛选）
  filterMenuExtra?: React.ReactNode;
  filterMenuExtraActive?: boolean;
  filterPresetMenu?: React.ReactNode;
  onRandomGame?: () => void;
  randomGameDisabled?: boolean;
  randomGameLoading?: boolean;
  actionButton?: React.ReactNode;
  extraButtons?: React.ReactNode;
  // 持久化存储键，传入后会自动保存和恢复排序设置
  storageKey?: string;
  // 批量选择
  batchMode?: boolean;
  onBatchModeChange?: (enabled: boolean) => void;
  selectedCount?: number;
  onSelectAll?: () => void;
  onClearSelection?: () => void;
  batchActions?: React.ReactNode;
}

export function FilterBar({
  searchQuery,
  onSearchChange,
  searchPlaceholder,
  disableStoredSearchQuery = false,
  sortBy,
  onSortByChange,
  sortOptions,
  sortOrder,
  onSortOrderChange,
  showSortField = false,
  onShowSortFieldChange,
  statusFilter,
  onStatusFilterChange,
  statusFilterInverted = false,
  onStatusFilterInvertedChange,
  statusOptions,
  filterMenuExtra,
  filterMenuExtraActive = false,
  filterPresetMenu,
  onRandomGame,
  randomGameDisabled = false,
  randomGameLoading = false,
  actionButton,
  extraButtons,
  storageKey,
  batchMode = false,
  onBatchModeChange,
  selectedCount,
  onSelectAll,
  onClearSelection,
  batchActions,
}: FilterBarProps) {
  const [initialized, setInitialized] = useState(false);
  const [draftSearchQuery, setDraftSearchQuery] = useState(searchQuery);
  const [isFilterDrawerOpen, setIsFilterDrawerOpen] = useState(false);
  const isComposingRef = useRef(false);
  const committedSearchQueryRef = useRef(searchQuery);
  const { t } = useTranslation();

  const finalSearchPlaceholder
    = searchPlaceholder || `${t("common.search")}...`;
  const activeFilterCount
    = (statusFilter ? 1 : 0) + (filterMenuExtraActive ? 1 : 0);

  useEffect(() => {
    committedSearchQueryRef.current = searchQuery;
    if (!isComposingRef.current) {
      setDraftSearchQuery(searchQuery);
    }
  }, [searchQuery]);

  // 初始化时从 localStorage 恢复所有设置
  useEffect(() => {
    if (storageKey && !initialized) {
      if (disableStoredSearchQuery) {
        setInitialized(true);
        return;
      }

      const savedSortBy = localStorage.getItem(`${storageKey}_sortBy`);
      const savedSortOrder = localStorage.getItem(`${storageKey}_sortOrder`);
      const savedSearchQuery = localStorage.getItem(
        `${storageKey}_searchQuery`,
      );
      const savedStatusFilter = localStorage.getItem(
        `${storageKey}_statusFilter`,
      );
      const savedStatusFilterInverted
        = localStorage.getItem(`${storageKey}_statusFilterInverted`) === "true";

      // 验证保存的 sortBy 是否在 sortOptions 中
      if (savedSortBy && sortOptions.some(opt => opt.value === savedSortBy)) {
        onSortByChange(savedSortBy);
      }

      if (
        savedSortOrder === enums.SortOrder.SortOrderAsc
        || savedSortOrder === enums.SortOrder.SortOrderDesc
      ) {
        onSortOrderChange(savedSortOrder as enums.SortOrder);
      }

      // 恢复搜索查询
      if (savedSearchQuery) {
        onSearchChange(savedSearchQuery);
      }

      // 恢复状态筛选
      if (savedStatusFilter && statusOptions && onStatusFilterChange) {
        // 验证保存的 statusFilter 是否在 statusOptions 中
        if (statusOptions.some(opt => opt.value === savedStatusFilter)) {
          onStatusFilterChange(savedStatusFilter as enums.GameStatus);
          if (savedStatusFilterInverted) {
            onStatusFilterInvertedChange?.(true);
          }
        }
      }

      setInitialized(true);
    }
  }, [
    disableStoredSearchQuery,
    storageKey,
    sortOptions,
    statusOptions,
    initialized,
  ]);

  // 处理搜索查询变更
  const commitSearchChange = (value: string) => {
    if (committedSearchQueryRef.current !== value) {
      committedSearchQueryRef.current = value;
      onSearchChange(value);
    }
    if (storageKey) {
      if (value) {
        localStorage.setItem(`${storageKey}_searchQuery`, value);
      }
      else {
        localStorage.removeItem(`${storageKey}_searchQuery`);
      }
    }
  };

  const handleSearchChange = (value: string) => {
    setDraftSearchQuery(value);
    if (!isComposingRef.current) {
      commitSearchChange(value);
    }
  };

  const handleSearchCompositionEnd = (
    event: React.CompositionEvent<HTMLInputElement>,
  ) => {
    isComposingRef.current = false;
    const value = event.currentTarget.value;
    setDraftSearchQuery(value);
    commitSearchChange(value);
  };

  // 处理状态筛选变更
  const handleStatusFilterChange = (value: enums.GameStatus | "") => {
    if (onStatusFilterChange) {
      onStatusFilterChange(value);
      if (storageKey) {
        if (value) {
          localStorage.setItem(`${storageKey}_statusFilter`, value);
          if (statusFilterInverted) {
            localStorage.setItem(`${storageKey}_statusFilterInverted`, "true");
          }
          else {
            localStorage.removeItem(`${storageKey}_statusFilterInverted`);
          }
        }
        else {
          localStorage.removeItem(`${storageKey}_statusFilter`);
          localStorage.removeItem(`${storageKey}_statusFilterInverted`);
        }
      }
      if (!value) {
        onStatusFilterInvertedChange?.(false);
      }
    }
  };

  const handleStatusFilterInvertedChange = (value: boolean) => {
    if (!statusFilter) {
      onStatusFilterInvertedChange?.(false);
      if (storageKey) {
        localStorage.removeItem(`${storageKey}_statusFilterInverted`);
      }
      return;
    }
    onStatusFilterInvertedChange?.(value);
    if (storageKey) {
      if (value) {
        localStorage.setItem(`${storageKey}_statusFilterInverted`, "true");
      }
      else {
        localStorage.removeItem(`${storageKey}_statusFilterInverted`);
      }
    }
  };

  // 处理排序方式变更
  const handleSortByChange = (value: string) => {
    onSortByChange(value);
    if (storageKey) {
      localStorage.setItem(`${storageKey}_sortBy`, value);
    }
  };

  // 处理排序顺序变更
  const handleSortOrderChange = (order: enums.SortOrder) => {
    onSortOrderChange(order);
    if (storageKey) {
      localStorage.setItem(`${storageKey}_sortOrder`, order);
    }
  };

  // 处理封面排序字段展示开关变更（持久化由父组件负责，写入 AppConfig）
  const handleShowSortFieldChange = (value: boolean) => {
    onShowSortFieldChange?.(value);
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-4 my-4">
      <div className="relative flex-1 max-w-md">
        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none transition-colors z-10">
          <div className="i-mdi-magnify text-lg text-brand-700 dark:text-brand-400" />
        </div>
        <input
          type="text"
          className="glass-input block w-full p-2 pl-10 text-sm text-brand-900 dark:text-white
                     bg-white dark:bg-brand-900
                     border border-brand-300 dark:border-brand-700
                     rounded-lg
                     placeholder:text-brand-700 dark:placeholder:text-brand-400
                     focus:ring-neutral-600 focus:border-neutral-600
                     dark:focus:ring-neutral-500 dark:focus:border-neutral-500"
          placeholder={finalSearchPlaceholder}
          value={draftSearchQuery}
          onChange={e => handleSearchChange(e.target.value)}
          onCompositionStart={() => {
            isComposingRef.current = true;
          }}
          onCompositionEnd={handleSearchCompositionEnd}
        />
      </div>

      <div className="flex items-center gap-2">
        {onRandomGame && (
          <button
            type="button"
            onClick={onRandomGame}
            disabled={randomGameDisabled || randomGameLoading}
            aria-label={t("filterBar.randomGame")}
            className="glass-btn-neutral flex items-center justify-center px-3 py-2 text-sm
                       text-brand-600 dark:text-brand-300
                       bg-brand-150 dark:bg-brand-700
                       border border-brand-200 dark:border-brand-700
                       rounded-lg hover:bg-brand-200 dark:hover:bg-brand-600
                       disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div
              className={`${
                randomGameLoading
                  ? "i-mdi-loading animate-spin"
                  : "i-mdi-dice-multiple-outline"
              } text-lg`}
              aria-hidden="true"
            />
          </button>
        )}

        {onBatchModeChange && (
          <button
            type="button"
            onClick={() => onBatchModeChange(!batchMode)}
            className={`glass-btn-neutral flex items-center gap-1.5 px-3 py-2 text-sm
                       ${
          batchMode
            ? "text-brand-900 dark:text-white bg-brand-200 dark:bg-brand-600 border-brand-300 dark:border-brand-500"
            : "text-brand-600 dark:text-brand-300 bg-brand-150 dark:bg-brand-700 border-brand-200 dark:border-brand-700"
          }
                       border rounded-lg hover:bg-brand-200 dark:hover:bg-brand-600`}
          >
            <div
              className={
                batchMode
                  ? "i-mdi-close-circle-outline text-lg"
                  : "i-mdi-checkbox-multiple-marked-outline text-lg"
              }
            />
          </button>
        )}

        <div className="relative inline-block">
          <button
            type="button"
            onClick={() => setIsFilterDrawerOpen(true)}
            aria-expanded={isFilterDrawerOpen}
            aria-haspopup="dialog"
            aria-label={`${t("filterBar.filters")} (${activeFilterCount})`}
            className="glass-btn-neutral flex items-center gap-2 px-3 py-2 text-sm
                       text-brand-700 dark:text-brand-300
                       bg-brand-150 dark:bg-brand-700
                       border border-brand-200 dark:border-brand-700
                       rounded-lg
                       hover:bg-brand-200 dark:hover:bg-brand-600"
          >
            <span className="relative inline-flex h-5 w-5 shrink-0 items-center justify-center">
              <span
                className="i-mdi-filter-variant text-lg"
                aria-hidden="true"
              />
              {activeFilterCount > 0 && (
                <span
                  className="absolute right-0 top-0 h-2 w-2 rounded-full bg-primary-500 ring-2 ring-brand-150 dark:ring-brand-700"
                  aria-hidden="true"
                />
              )}
            </span>
            <span>{t("filterBar.filters")}</span>
          </button>

          <BetterDrawer
            isOpen={isFilterDrawerOpen}
            onOpenChange={setIsFilterDrawerOpen}
            title={t("filterBar.filters")}
            closeLabel={t("common.cancel")}
            placement="right"
            bodyClassName="p-2"
            topOffset={TOPBAR_HEIGHT}
          >
            {filterMenuExtra && (
              <div className="w-full min-w-0 px-2 py-1.5">
                {filterMenuExtra}
              </div>
            )}

            {statusOptions && onStatusFilterChange && (
              <>
                <div className="px-2 py-1.5">
                  <div className="mb-1.5 flex items-center justify-between gap-2">
                    <div className="text-xs font-medium text-brand-400 dark:text-brand-500">
                      {t("filterBar.status")}
                    </div>
                    {onStatusFilterInvertedChange && (
                      <button
                        type="button"
                        disabled={!statusFilter}
                        aria-label={t("filterBar.invertStatusFilter")}
                        onClick={() =>
                          handleStatusFilterInvertedChange(
                            !statusFilterInverted,
                          )}
                        className={`inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-1.5 text-[11px] font-medium transition-colors
                          ${
                      statusFilter && statusFilterInverted
                        ? "text-neutral-600 hover:text-neutral-700 dark:text-brand-200 dark:hover:text-white"
                        : "text-brand-400 hover:bg-brand-50 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-500 dark:hover:bg-brand-700/60 dark:hover:text-brand-300"
                      }`}
                      >
                        <div className="i-mdi-swap-horizontal text-sm" />
                        {t("filterBar.invert")}
                      </button>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {statusOptions.map(option => (
                      <button
                        key={`status-${option.value || "all"}`}
                        type="button"
                        onClick={() => handleStatusFilterChange(option.value)}
                        className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors
                          ${
                      (statusFilter || "") === option.value
                        ? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-200 ring-1 ring-brand-300 dark:ring-brand-500"
                        : "bg-brand-50 text-brand-500 hover:bg-brand-100 dark:bg-brand-900/50 dark:text-brand-400 dark:hover:bg-brand-700/70"
                      }`}
                      >
                        {option.label}
                      </button>
                    ))}
                  </div>
                </div>
              </>
            )}

            {filterPresetMenu && (
              <>
                {(filterMenuExtra
                  || (statusOptions && onStatusFilterChange)) && (
                  <div className="my-1 border-t border-brand-200 dark:border-brand-700" />
                )}
                <div className="w-full min-w-0 px-2 py-1.5">
                  {filterPresetMenu}
                </div>
              </>
            )}

            {(filterMenuExtra
              || (statusOptions && onStatusFilterChange)
              || filterPresetMenu) && (
              <div className="my-1 border-t border-brand-200 dark:border-brand-700" />
            )}

            <div className="px-2 py-1.5">
              <div className="mb-1.5 text-xs font-medium text-brand-400 dark:text-brand-500">
                {t("filterBar.sortBy")}
              </div>
              <div className="space-y-1">
                {sortOptions.map(option => (
                  <button
                    key={`sort-${option.value}`}
                    type="button"
                    onClick={() => handleSortByChange(option.value)}
                    className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-sm transition-colors
                      ${
                  sortBy === option.value
                    ? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-200"
                    : "text-brand-600 hover:bg-brand-50 dark:text-brand-300 dark:hover:bg-brand-700/70"
                  }`}
                  >
                    <span>{option.label}</span>
                    {sortBy === option.value && (
                      <div className="i-mdi-check text-base" />
                    )}
                  </button>
                ))}
              </div>
            </div>

            <div className="px-2 pb-1.5 pt-0.5">
              <div className="mb-1.5 text-xs font-medium text-brand-400 dark:text-brand-500">
                {t("filterBar.sortDirection")}
              </div>
              <div className="grid grid-cols-2 gap-1.5">
                <button
                  type="button"
                  onClick={() =>
                    handleSortOrderChange(enums.SortOrder.SortOrderAsc)}
                  className={`flex items-center justify-center gap-1.5 rounded-lg px-2.5 py-2 text-xs font-medium transition-colors
                    ${
    sortOrder === enums.SortOrder.SortOrderAsc
      ? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-200"
      : "bg-brand-50 text-brand-500 hover:bg-brand-100 dark:bg-brand-900/50 dark:text-brand-400 dark:hover:bg-brand-700/70"
    }`}
                >
                  <div className="i-mdi-sort-ascending text-base" />
                  {t("filterBar.sortAsc")}
                </button>
                <button
                  type="button"
                  onClick={() =>
                    handleSortOrderChange(enums.SortOrder.SortOrderDesc)}
                  className={`flex items-center justify-center gap-1.5 rounded-lg px-2.5 py-2 text-xs font-medium transition-colors
                    ${
    sortOrder === enums.SortOrder.SortOrderDesc
      ? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-200"
      : "bg-brand-50 text-brand-500 hover:bg-brand-100 dark:bg-brand-900/50 dark:text-brand-400 dark:hover:bg-brand-700/70"
    }`}
                >
                  <div className="i-mdi-sort-descending text-base" />
                  {t("filterBar.sortDesc")}
                </button>
              </div>
            </div>

            {onShowSortFieldChange && (
              <div className="px-2 pb-1.5 pt-0.5">
                <label
                  htmlFor="filter-bar-show-sort-field"
                  className="flex items-center justify-between gap-3 cursor-pointer"
                >
                  <span className="text-xs font-medium text-brand-400 dark:text-brand-500">
                    {t("filterBar.showSortField")}
                  </span>
                  <BetterSwitch
                    id="filter-bar-show-sort-field"
                    checked={showSortField}
                    onCheckedChange={handleShowSortFieldChange}
                  />
                </label>
              </div>
            )}
          </BetterDrawer>
        </div>

        {extraButtons}
        {actionButton}
      </div>

      {/* 批量操作按钮 - 第二行 */}
      {batchMode && (
        <div className="w-full bg-gradient-to-r from-brand-50 to-brand-50/50 dark:from-brand-800/50 dark:to-brand-900/30 border border-brand-200 dark:border-brand-700/50 rounded-lg px-3 py-2 flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1.5 flex-wrap">
            {onSelectAll && (
              <button
                type="button"
                onClick={onSelectAll}
                className="px-2.5 py-1 text-xs font-medium text-brand-600 dark:text-brand-300
                           bg-white dark:bg-brand-700/60 border border-brand-200 dark:border-brand-600
                           rounded-md hover:bg-brand-50 dark:hover:bg-brand-700 transition-colors"
              >
                {t("common.selectAll")}
              </button>
            )}
            {onClearSelection && (
              <button
                type="button"
                onClick={onClearSelection}
                className="px-2.5 py-1 text-xs font-medium text-brand-600 dark:text-brand-300
                           bg-white dark:bg-brand-700/60 border border-brand-200 dark:border-brand-600
                           rounded-md hover:bg-brand-50 dark:hover:bg-brand-700 transition-colors"
              >
                {t("common.clearStore")}
              </button>
            )}
          </div>

          {typeof selectedCount === "number" && (
            <div className="px-2.5 py-1 text-xs font-medium text-brand-700 dark:text-brand-300 bg-white dark:bg-brand-700/60 border border-brand-200 dark:border-brand-600 rounded-md">
              {t("common.selected")}
              {" "}
              <span className="font-semibold ml-1">{selectedCount}</span>
            </div>
          )}

          {batchActions && (
            <div className="flex items-center gap-1.5 ml-auto">
              {batchActions}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
