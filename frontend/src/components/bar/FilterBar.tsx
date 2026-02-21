import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BetterSelect } from "../ui/BetterSelect";

interface SortOption {
  label: string;
  value: string;
}

interface FilterOption {
  label: string;
  value: string;
}

interface FilterBarProps {
  searchQuery: string;
  onSearchChange: (value: string) => void;
  searchPlaceholder?: string;
  sortBy: string;
  onSortByChange: (value: string) => void;
  sortOptions: SortOption[];
  sortOrder: "asc" | "desc";
  onSortOrderChange: (order: "asc" | "desc") => void;
  // 状态筛选
  statusFilter?: string;
  onStatusFilterChange?: (value: string) => void;
  statusOptions?: FilterOption[];
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
  sortBy,
  onSortByChange,
  sortOptions,
  sortOrder,
  onSortOrderChange,
  statusFilter,
  onStatusFilterChange,
  statusOptions,
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
  const { t } = useTranslation();

  const finalSearchPlaceholder = searchPlaceholder || `${t("common.search")}...`;

  // 初始化时从 localStorage 恢复所有设置

  useEffect(() => {
    if (storageKey && !initialized) {
      const savedSortBy = localStorage.getItem(`${storageKey}_sortBy`);
      const savedSortOrder = localStorage.getItem(`${storageKey}_sortOrder`);
      const savedSearchQuery = localStorage.getItem(`${storageKey}_searchQuery`);
      const savedStatusFilter = localStorage.getItem(`${storageKey}_statusFilter`);

      // 验证保存的 sortBy 是否在 sortOptions 中
      if (savedSortBy && sortOptions.some(opt => opt.value === savedSortBy)) {
        onSortByChange(savedSortBy);
      }

      if (savedSortOrder === "asc" || savedSortOrder === "desc") {
        onSortOrderChange(savedSortOrder);
      }

      // 恢复搜索查询
      if (savedSearchQuery) {
        onSearchChange(savedSearchQuery);
      }

      // 恢复状态筛选
      if (savedStatusFilter && statusOptions && onStatusFilterChange) {
        // 验证保存的 statusFilter 是否在 statusOptions 中
        if (statusOptions.some(opt => opt.value === savedStatusFilter)) {
          onStatusFilterChange(savedStatusFilter);
        }
      }

      setInitialized(true);
    }
  }, [storageKey, sortOptions, statusOptions, initialized]);

  // 处理搜索查询变更
  const handleSearchChange = (value: string) => {
    onSearchChange(value);
    if (storageKey) {
      if (value) {
        localStorage.setItem(`${storageKey}_searchQuery`, value);
      }
      else {
        localStorage.removeItem(`${storageKey}_searchQuery`);
      }
    }
  };

  // 处理状态筛选变更
  const handleStatusFilterChange = (value: string) => {
    if (onStatusFilterChange) {
      onStatusFilterChange(value);
      if (storageKey) {
        if (value) {
          localStorage.setItem(`${storageKey}_statusFilter`, value);
        }
        else {
          localStorage.removeItem(`${storageKey}_statusFilter`);
        }
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
  const handleSortOrderChange = (order: "asc" | "desc") => {
    onSortOrderChange(order);
    if (storageKey) {
      localStorage.setItem(`${storageKey}_sortOrder`, order);
    }
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-4 my-4">
      <div className="relative flex-1 max-w-md">
        <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
          <div className="i-mdi-magnify text-brand-700" />
        </div>
        <input
          type="text"
          className="glass-input block w-auto p-2 pl-10 text-sm text-brand-900 dark:text-white
                     bg-white dark:bg-brand-900
                     border border-brand-300 dark:border-brand-700
                     rounded-lg
                     placeholder:text-brand-700 dark:placeholder:text-brand-400
                     focus:ring-neutral-600 focus:border-neutral-600
                     dark:focus:ring-neutral-500 dark:focus:border-neutral-500"
          placeholder={finalSearchPlaceholder}
          value={searchQuery}
          onChange={e => handleSearchChange(e.target.value)}
        />
      </div>

      <div className="flex items-center gap-2">
        {onBatchModeChange && (
          <button
            type="button"
            onClick={() => onBatchModeChange(!batchMode)}
            className={`glass-panel flex items-center gap-1.5 px-3 py-2 text-sm
                       ${batchMode
            ? "text-brand-900 dark:text-white bg-brand-100 dark:bg-brand-700 border-brand-300 dark:border-brand-600"
            : "text-brand-500 dark:text-brand-400 bg-white dark:bg-brand-800 border-brand-200 dark:border-brand-700"}
                       border rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700`}
            title={batchMode ? t("filterBar.exitBatchSelection") : t("filterBar.enterBatchSelection")}
          >
            <div className={batchMode ? "i-mdi-close-circle-outline text-lg" : "i-mdi-checkbox-multiple-marked-outline text-lg"} />
          </button>
        )}

        {/* 状态筛选 */}
        {statusOptions && onStatusFilterChange && (
          <BetterSelect
            value={statusFilter || ""}
            onChange={handleStatusFilterChange}
            options={statusOptions}
            className="min-w-[120px]"
          />
        )}

        <BetterSelect
          value={sortBy}
          onChange={handleSortByChange}
          options={sortOptions}
          className="min-w-[120px]"
        />

        <button
          type="button"
          onClick={() => handleSortOrderChange(sortOrder === "asc" ? "desc" : "asc")}
          className="glass-panel p-2
                     text-brand-500 dark:text-brand-400
                     hover:text-brand-900 dark:hover:text-white
                     bg-white dark:bg-brand-800
                     border border-brand-200 dark:border-brand-700
                     rounded-lg
                     hover:bg-brand-100 dark:hover:bg-brand-700"
          title={sortOrder === "asc" ? t("filterBar.sortAsc") : t("filterBar.sortDesc")}
        >
          <div className={sortOrder === "asc" ? "i-mdi-sort-ascending text-xl" : "i-mdi-sort-descending text-xl"} />
        </button>

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
